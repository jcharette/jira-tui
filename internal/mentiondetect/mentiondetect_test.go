package mentiondetect

import "testing"

func TestDetectMentions(t *testing.T) {
	mentions := Detect("Please ask @Jane and @ops.team_1, not email@example.test")

	want := []Mention{
		{Query: "Jane", Start: 11, End: 16},
		{Query: "ops.team_1", Start: 21, End: 32},
	}
	if len(mentions) != len(want) {
		t.Fatalf("mentions = %#v", mentions)
	}
	for index := range want {
		if mentions[index] != want[index] {
			t.Fatalf("mentions[%d] = %#v, want %#v", index, mentions[index], want[index])
		}
	}
}

func TestDetectMentionsIgnoresEmails(t *testing.T) {
	mentions := Detect("Contact ops@example.test")
	if len(mentions) != 0 {
		t.Fatalf("mentions = %#v", mentions)
	}
}
