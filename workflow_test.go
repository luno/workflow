package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

// Ensures that workflow.Workflow always implements workflow.API
var _ workflow.API[MyType, status] = (*workflow.Workflow[MyType, status])(nil)

type MyType struct {
	UserID      int64
	Profile     string
	Email       string
	Cellphone   string
	OTP         int
	OTPVerified bool
}

func (m MyType) ForeignID() string {
	return strconv.FormatInt(m.UserID, 10)
}

type status int

const (
	StatusUnknown                  status = 0
	StatusInitiated                status = 1
	StatusProfileCreated           status = 2
	StatusEmailConfirmationSent    status = 3
	StatusEmailVerified            status = 4
	StatusCellphoneNumberSubmitted status = 5
	StatusOTPSent                  status = 6
	StatusOTPVerified              status = 7
	StatusCompleted                status = 8

	StatusStart  status = 9
	StatusMiddle status = 10
	StatusEnd    status = 11
)

func (s status) String() string {
	switch s {
	case StatusInitiated:
		return "Initiated"
	case StatusProfileCreated:
		return "Profile Created"
	case StatusEmailConfirmationSent:
		return "Email Confirmation Sent"
	case StatusEmailVerified:
		return "Email Verified"
	case StatusCellphoneNumberSubmitted:
		return "Cellphone Number Submitted"
	case StatusOTPSent:
		return "OTP Sent"
	case StatusOTPVerified:
		return "OTP Verified"
	case StatusCompleted:
		return "Completed"
	case StatusStart:
		return "Start"
	case StatusMiddle:
		return "Middle"
	case StatusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

type ExternalEmailVerified struct {
	IsVerified bool
}

type ExternalCellPhoneSubmitted struct {
	DialingCode string
	Number      string
}

type ExternalOTP struct {
	OTPCode int
}

func TestWorkflowAcceptanceTest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	b := workflow.NewBuilder[MyType, status]("user sign up")
	b.AddStep(StatusInitiated, createProfile, StatusProfileCreated)
	b.AddStep(StatusProfileCreated, sendEmailConfirmation, StatusEmailConfirmationSent)
	b.AddCallback(StatusEmailConfirmationSent, emailVerifiedCallback, StatusEmailVerified)
	b.AddCallback(StatusEmailVerified, cellphoneNumberCallback, StatusCellphoneNumberSubmitted)
	b.AddStep(StatusCellphoneNumberSubmitted, sendOTP, StatusOTPSent).WithOptions(workflow.ParallelCount(2))
	b.AddCallback(StatusOTPSent, otpCallback, StatusOTPVerified)
	b.AddTimeout(StatusOTPVerified, workflow.DurationTimerFunc[MyType, status](time.Hour), waitForAccountCoolDown, StatusCompleted)

	recordStore := memrecordstore.New()
	clock := clock_testing.NewFakeClock(time.Now())
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
		workflow.WithClock(clock),
		workflow.WithDebugMode(),
	)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := strconv.FormatInt(expectedUserID, 10)

	mt := MyType{
		UserID: expectedUserID,
	}

	runID, err := wf.Trigger(ctx, fid, StatusInitiated, workflow.WithInitialValue[MyType, status](&mt))
	require.Nil(t, err)

	// Once in the correct status, trigger third party callbacks
	workflow.TriggerCallbackOn(t, wf, fid, runID, StatusEmailConfirmationSent, ExternalEmailVerified{
		IsVerified: true,
	})

	workflow.TriggerCallbackOn(t, wf, fid, runID, StatusEmailVerified, ExternalCellPhoneSubmitted{
		DialingCode: "+44",
		Number:      "7467623292",
	})

	workflow.TriggerCallbackOn(t, wf, fid, runID, StatusOTPSent, ExternalOTP{
		OTPCode: expectedOTP,
	})

	workflow.AwaitTimeoutInsert(t, wf, fid, runID, StatusOTPVerified)

	// Advance time forward by one hour to trigger the timeout
	clock.Step(time.Hour)

	_, err = wf.Await(ctx, fid, runID, StatusCompleted)
	require.Nil(t, err)

	r, err := recordStore.Latest(ctx, "user sign up", fid)
	require.Nil(t, err)
	require.Equal(t, int(expectedFinalStatus), r.Status)

	var actual MyType
	err = workflow.Unmarshal(r.Object, &actual)
	require.Nil(t, err)

	require.Equal(t, expectedUserID, actual.UserID)
	require.Equal(t, strconv.FormatInt(expectedUserID, 10), actual.ForeignID())
	require.Equal(t, expectedProfile, actual.Profile)
	require.Equal(t, expectedEmail, actual.Email)
	require.Equal(t, expectedCellphone, actual.Cellphone)
	require.Equal(t, expectedOTP, actual.OTP)
	require.Equal(t, expectedOTPVerified, actual.OTPVerified)
}

func BenchmarkWorkflow(b *testing.B) {
	b.Run("1", func(b *testing.B) {
		benchmarkWorkflow(b, 1)
	})
	b.Run("5", func(b *testing.B) {
		benchmarkWorkflow(b, 5)
	})
	b.Run("10", func(b *testing.B) {
		benchmarkWorkflow(b, 10)
	})
}

func benchmarkWorkflow(b *testing.B, numberOfSteps int) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(func() {
		cancel()
	})

	bldr := workflow.NewBuilder[MyType, status]("benchmark")

	for i := range numberOfSteps {
		bldr.AddStep(status(i), func(ctx context.Context, r *workflow.Run[MyType, status]) (status, error) {
			return status(i + 1), nil
		}, status(i+1))
	}

	recordStore := memrecordstore.New()
	clock := clock_testing.NewFakeClock(time.Now())
	wf := bldr.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithOutboxPollingFrequency(1*time.Nanosecond),
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(1*time.Nanosecond),
		),
	)

	wf.Run(ctx)
	b.Cleanup(wf.Stop)

	fid := strconv.FormatInt(expectedUserID, 10)
	mt := MyType{
		UserID: expectedUserID,
	}
	for range b.N {
		_, err := wf.Trigger(ctx, fid, 0, workflow.WithInitialValue[MyType, status](&mt))
		if err != nil {
			b.Fatal(err)
		}

		workflow.Require(b, wf, fid, status(numberOfSteps), MyType{
			UserID: expectedUserID,
		})
	}
}

func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	b := workflow.NewBuilder[MyType, status]("user sign up")

	b.AddStep(StatusInitiated, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		return StatusProfileCreated, nil
	}, StatusProfileCreated)

	b.AddTimeout(StatusProfileCreated, workflow.DurationTimerFunc[MyType, status](time.Hour), func(ctx context.Context, t *workflow.Run[MyType, status], now time.Time) (status, error) {
		return StatusCompleted, nil
	}, StatusCompleted).WithOptions(
		workflow.PollingFrequency(100 * time.Millisecond),
	)

	recordStore := memrecordstore.New()
	clock := clock_testing.NewFakeClock(time.Now())
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
		workflow.WithClock(clock),
	)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	start := time.Now()

	runID, err := wf.Trigger(ctx, "example", StatusInitiated)
	require.Nil(t, err)

	workflow.AwaitTimeoutInsert(t, wf, "example", runID, StatusProfileCreated)

	// Advance time forward by one hour to trigger the timeout
	clock.Step(time.Hour)

	_, err = wf.Await(ctx, "example", runID, StatusCompleted)
	require.Nil(t, err)

	end := time.Now()

	require.True(t, end.Sub(start) < 1*time.Second)
}

var (
	expectedUserID      int64 = 984892374983743
	expectedFinalStatus       = StatusCompleted
	expectedProfile           = "Andrew Wormald"
	expectedEmail             = "andreww@luno.com"
	expectedCellphone         = "+44 7467623292"
	expectedOTP               = 345345
	expectedOTPVerified       = true
)

func createProfile(ctx context.Context, mt *workflow.Run[MyType, status]) (status, error) {
	mt.Object.Profile = "Andrew Wormald"
	return StatusProfileCreated, nil
}

func sendEmailConfirmation(ctx context.Context, mt *workflow.Run[MyType, status]) (status, error) {
	return StatusEmailConfirmationSent, nil
}

func emailVerifiedCallback(ctx context.Context, mt *workflow.Run[MyType, status], r io.Reader) (status, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	var ev ExternalEmailVerified
	err = json.Unmarshal(b, &ev)
	if err != nil {
		return 0, err
	}

	if !ev.IsVerified {
		// Skip callback
		return 0, nil
	}

	mt.Object.Email = "andreww@luno.com"
	return StatusEmailVerified, nil
}

func cellphoneNumberCallback(ctx context.Context, mt *workflow.Run[MyType, status], r io.Reader) (status, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	var ev ExternalCellPhoneSubmitted
	err = json.Unmarshal(b, &ev)
	if err != nil {
		return 0, err
	}

	if ev.DialingCode != "" && ev.Number != "" {
		mt.Object.Cellphone = fmt.Sprintf("%v %v", ev.DialingCode, ev.Number)
	}

	return StatusCellphoneNumberSubmitted, nil
}

func sendOTP(ctx context.Context, mt *workflow.Run[MyType, status]) (status, error) {
	mt.Object.OTP = expectedOTP
	return StatusOTPSent, nil
}

func otpCallback(ctx context.Context, mt *workflow.Run[MyType, status], r io.Reader) (status, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	var otp ExternalOTP
	err = json.Unmarshal(b, &otp)
	if err != nil {
		return 0, err
	}

	if otp.OTPCode == expectedOTP {
		mt.Object.OTPVerified = true
	}

	return StatusOTPVerified, nil
}

func waitForAccountCoolDown(ctx context.Context, mt *workflow.Run[MyType, status], now time.Time) (status, error) {
	return StatusCompleted, nil
}

func TestWorkflow_ErrWorkflowNotRunning(t *testing.T) {
	b := workflow.NewBuilder[MyType, status]("sync users")

	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd)

	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	_, err := wf.Trigger(ctx, "andrew", StatusStart)
	require.True(t, errors.Is(err, workflow.ErrWorkflowNotRunning))

	err = wf.Schedule("andrew", StatusStart, "@monthly")
	require.True(t, errors.Is(err, workflow.ErrWorkflowNotRunning))
}

func TestWorkflow_TestingRequire(t *testing.T) {
	b := workflow.NewBuilder[MyType, status]("sync users")

	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		t.Object.Email = "andrew@workflow.com"
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		t.Object.Cellphone = "+44 349 8594"
		return StatusEnd, nil
	}, StatusEnd)

	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	foreignID := "andrew"
	_, err := wf.Trigger(ctx, foreignID, StatusStart)
	require.Nil(t, err)

	expected := MyType{
		Email: "andrew@workflow.com",
	}
	workflow.Require(t, wf, foreignID, StatusMiddle, expected)

	expected = MyType{
		Email:     "andrew@workflow.com",
		Cellphone: "+44 349 8594",
	}
	workflow.Require(t, wf, foreignID, StatusEnd, expected)
}

func TestTimeTimerFunc(t *testing.T) {
	type YinYang struct {
		Yin  bool
		Yang bool
	}

	b := workflow.NewBuilder[YinYang, status]("timer_func")

	launchDate := time.Date(1992, time.April, 9, 0, 0, 0, 0, time.UTC)
	b.AddTimeout(StatusStart,
		workflow.TimeTimerFunc[YinYang, status](launchDate),
		func(ctx context.Context, t *workflow.Run[YinYang, status], now time.Time) (status, error) {
			t.Object.Yin = true
			t.Object.Yang = true
			return StatusEnd, nil
		},
		StatusEnd,
	)

	now := time.Date(1991, time.December, 25, 8, 30, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)

	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	runID, err := wf.Trigger(ctx, "Andrew Wormald", StatusStart)
	require.Nil(t, err)

	workflow.AwaitTimeoutInsert(t, wf, "Andrew Wormald", runID, StatusStart)

	clock.SetTime(launchDate)

	expected := YinYang{
		Yin:  true,
		Yang: true,
	}
	workflow.Require(t, wf, "Andrew Wormald", StatusEnd, expected)
}

func TestConnector(t *testing.T) {
	type typeX struct {
		Val string
	}

	events := []workflow.ConnectorEvent{
		{
			ID:        "1",
			ForeignID: "SDFJKH-SDKFJHBSD-SDKFJBS",
			Type:      "2",
		},
		{
			ID:        "2",
			ForeignID: "XCVXCM-EIXCASDBJ-SDFBJKZ",
			Type:      "2",
		},
	}
	connector := memstreamer.NewConnector(events)

	buidler := workflow.NewBuilder[typeX, status]("workflow")
	buidler.AddConnector(
		"my-test-connector",
		connector,
		func(ctx context.Context, w *workflow.Workflow[typeX, status], e *workflow.ConnectorEvent) error {
			_, err := w.Trigger(ctx, e.ForeignID, StatusStart, workflow.WithInitialValue[typeX, status](&typeX{
				Val: "trigger set value",
			}))
			if err != nil {
				return err
			}

			return nil
		},
	)

	buidler.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[typeX, status]) (status, error) {
		r.Object.Val = "workflow step set value"
		return StatusEnd, nil
	}, StatusEnd)

	w := buidler.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	for _, event := range events {
		workflow.Require(t, w, event.ForeignID, StatusStart, typeX{
			Val: "trigger set value",
		})

		workflow.Require(t, w, event.ForeignID, StatusEnd, typeX{
			Val: "workflow step set value",
		})
	}
}

func TestStepConsumerLag(t *testing.T) {
	fixedNowTime := time.Date(2023, time.April, 9, 8, 30, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(fixedNowTime)
	lagAmount := time.Hour

	type TimeWatcher struct {
		StartTime   time.Time
		ConsumeTime time.Time
	}

	b := workflow.NewBuilder[TimeWatcher, status]("step consumer lag")
	b.AddStep(
		StatusStart,
		func(ctx context.Context, t *workflow.Run[TimeWatcher, status]) (status, error) {
			t.Object.ConsumeTime = clock.Now()

			return StatusEnd, nil
		},
		StatusEnd,
	).WithOptions(
		workflow.ConsumeLag(lagAmount),
	)

	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(memstreamer.WithClock(clock)),
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithDebugMode(),
	)

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	foreignID := "1"
	_, err := wf.Trigger(ctx, foreignID, StatusStart, workflow.WithInitialValue[TimeWatcher, status](&TimeWatcher{
		StartTime: clock.Now(),
	}))
	require.Nil(t, err)

	time.Sleep(time.Second)

	latest, err := recordStore.Latest(ctx, wf.Name, foreignID)
	require.Nil(t, err)

	// Ensure that the record has not been consumer or updated
	require.Equal(t, int64(1), latest.ID)
	require.Equal(t, int(StatusStart), latest.Status)

	clock.Step(lagAmount)

	workflow.Require(t, wf, foreignID, StatusEnd, TimeWatcher{
		StartTime:   fixedNowTime,
		ConsumeTime: fixedNowTime.Add(lagAmount),
	})
}
