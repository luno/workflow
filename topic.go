package workflow

import (
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
		strconv.FormatInt(int64(statusType), 10),
	}, topicSeparator)
}

func DeleteTopic(workflowName string) string {
	name := strings.ReplaceAll(workflowName, " ", emptySpaceReplacement)
	return strings.Join([]string{
		name,
		"delete",
	}, topicSeparator)
}

func RunStateChangeTopic(workflowName string) string {
	name := strings.ReplaceAll(workflowName, " ", emptySpaceReplacement)
	return strings.Join([]string{
		name,
		"run-state-change",
	}, topicSeparator)
}
