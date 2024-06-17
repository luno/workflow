package workflow

import (
	"fmt"
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

func DeleteTopic(workflowName string) string {
	name := strings.ReplaceAll(workflowName, " ", emptySpaceReplacement)
	return strings.Join([]string{
		name,
		"delete",
	}, topicSeparator)
}
