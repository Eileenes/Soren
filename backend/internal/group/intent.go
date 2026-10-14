package group

import (
	"regexp"
	"strings"
)

// Only an explicit invitation enables multiple rounds. Ordinary questions,
// including questions about a discussion, remain a single round by default.
var discussionInvitation = regexp.MustCompile(`(?i)(请|让)?(大家|你们|各位|所有人|所有\s*bot)(一起|互相|相互|来|继续|围绕[^。！？\n]{1,32}|就[^。！？\n]{1,32}){0,3}(讨论|辩论|探讨)|(请|来|一起|互相|相互|继续)(讨论|辩论|探讨)|(讨论|探讨)(一下|下吧|起来)|\b(let'?s|please|everyone|all bots)\s+(discuss|debate|brainstorm)\b|\bcontinue (the )?(discussion|debate)\b`)
var singleRoundRequest = regexp.MustCompile(`(?i)(不要|别|不用|无需|停止|结束|不必).{0,8}(讨论|辩论|探讨)|只.{0,8}(回复|回答|一轮)|\b(don'?t|do not|stop|no need to)\s+(discuss|debate)|\b(one|single) round\b`)
var discussionReference = regexp.MustCompile(`^(的|了|过|到|完|结束|结果|进展|结论)`)
var discussionDirective = regexp.MustCompile(`^(请|让)|(一起|互相|相互|来|继续)(讨论|辩论|探讨)`)
var discussionQuestion = regexp.MustCompile(`(?i)^(为什么|为何|怎么|如何|什么时候|何时|是否|总结|回顾|请总结|请回顾|\b(what|when|why|how|who|did|does|has|have)\b)`)

func invitesDiscussion(content string) bool {
	// Match each clause separately: asking about an earlier discussion must not
	// spend another three rounds, but a later explicit invitation still can.
	clauses := strings.FieldsFunc(content, func(r rune) bool {
		return strings.ContainsRune("。！？\n；;，,：:.!?", r)
	})
	for _, clause := range clauses {
		for _, match := range discussionInvitation.FindAllStringIndex(clause, -1) {
			before := strings.TrimSpace(clause[:match[0]])
			after := strings.TrimSpace(clause[match[1]:])
			invitation := clause[match[0]:match[1]]
			// "请讨论过去的经验" is still an instruction; the reference guard
			// applies to unqualified phrases such as "大家讨论过这个方案吗".
			if discussionQuestion.MatchString(before) || (discussionReference.MatchString(after) && !discussionDirective.MatchString(invitation)) {
				continue
			}
			return true
		}
	}
	return false
}

func resolveMode(content, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" && requested != ModeMention && requested != ModeAuto {
		return "", badRequest("发言模式无效")
	}
	if singleRoundRequest.MatchString(content) {
		return ModeMention, nil
	}
	// Explicit protocol mode remains supported for existing integrations. The
	// product UI no longer persists a mode; each user message decides its scope.
	if requested != "" {
		return requested, nil
	}
	if invitesDiscussion(content) {
		return ModeAuto, nil
	}
	return ModeMention, nil
}

func isContinuation(content string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(content), "。！.! ")) {
	case "继续", "继续吧", "请继续", "好", "好的", "可以", "yes", "continue":
		return true
	}
	return false
}
