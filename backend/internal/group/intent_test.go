package group

import "testing"

func TestOnlyExplicitDiscussionInvitationsUseMultipleRounds(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"大家觉得如何？", ModeMention}, {"@产品 给个建议", ModeMention},
		{"请大家讨论这个方案", ModeAuto}, {"让大家围绕成本讨论", ModeAuto},
		{"你们互相讨论一下", ModeAuto}, {"继续讨论", ModeAuto},
		{"Please discuss the launch", ModeAuto}, {"Let's brainstorm", ModeAuto},
		{"不要讨论，只回答一轮", ModeMention}, {"讨论会几点开始？", ModeMention},
		{"大家讨论的结果是什么？", ModeMention},
		{"大家讨论过这个方案吗？", ModeMention},
		{"你们讨论到哪里了？", ModeMention},
		{"大家讨论结束了吗？", ModeMention},
		{"What did everyone discuss yesterday?", ModeMention},
		{"为什么让大家讨论这个方案？", ModeMention},
		{"请大家讨论过去一年的经营情况", ModeAuto},
		{"请讨论到期续费方案", ModeAuto},
		{"请问大家讨论的结果是什么？", ModeMention},
		{"大家讨论的结果我看了，请大家继续讨论定价。", ModeAuto},
		{"@产品 @研发 请讨论一下发布计划", ModeAuto},
		{"请大家讨论，但只讨论一轮", ModeMention},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := resolveMode(tc.text, "")
			if err != nil || got != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}
