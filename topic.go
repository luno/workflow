package workflow

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	topicSeparator        = "-"
	emptySpaceReplacement = "_"
)

func Topic(workflowName string, statusType int) string {
	name := strings.ReplaceAll(workflowName, " ", emptySpaceReplacement)
	return strings.Join([]string{
		name,
		fmt.Sprintf("%v", statusType),
	}, topicSeparator)
}

func ParseTopic(topic string) (workflowName string, statusType int, err error) {
	parts := strings.Split(topic, topicSeparator)
	if len(parts) < 2 {
		return "", 0, nil
	}

	status, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, err
	}

	workflowName = strings.ReplaceAll(parts[0], emptySpaceReplacement, " ")
	return workflowName, int(status), nil
}
