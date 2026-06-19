package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func TestCommentComposerConfirmsAndPostsComment(t *testing.T) {
	store := newFakeActiveViewStore()
	model := NewModel(&fakeIssueSearcher{
		addedComment: jira.Comment{ID: "10002", Author: "Current User", Body: "Please review"},
		comments:     []jira.Comment{{ID: "10002", Author: "Current User", Body: "Please review"}},
	}, "project = ABC", WithActiveViewStore(store, "https://example.atlassian.net"))
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
			Description: "No links here.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {},
	}
	focusDetailSectionForTest(t, &model, "Comments")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if next.mode != modeComment {
		t.Fatalf("mode = %v", next.mode)
	}

	for _, key := range []string{"P", "l", "e", "a", "s", "e", "-", "r", "e", "v", "i", "e", "w"} {
		updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		next = updated.(Model)
	}
	if next.commentDraft != "Please-review" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if !next.commentConfirm {
		t.Fatal("expected confirmation state")
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected add comment command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)

	resultMsg := next.waitForWorkerResult()()
	updated, cmd = next.Update(resultMsg)
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode after add = %v", next.mode)
	}
	if next.commentDraft != "" {
		t.Fatalf("commentDraft after add = %q", next.commentDraft)
	}
	if !strings.Contains(next.detailNotice, "Comment posted") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if cmd == nil {
		t.Fatal("expected comments refresh command")
	}
	if !next.commentsLoading {
		t.Fatal("expected comments refresh to start")
	}
	if _, ok := next.comments["ABC-1"]; ok {
		t.Fatalf("expected stale comment cache to be cleared: %#v", next.comments["ABC-1"])
	}
	if _, ok := next.cachedIssueComments("ABC-1"); ok {
		t.Fatal("expected retained comment cache to be invalidated")
	}
	if store.deletedComments != "ABC-1" {
		t.Fatalf("deleted persistent comments = %q", store.deletedComments)
	}
}

func TestCommentSectionFocusEditsSelectedComment(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {
			{ID: "10001", Author: "Jane", Body: "First comment"},
			{ID: "10002", Author: "Jon", Body: "Second comment"},
		},
	}
	focusDetailSectionForTest(t, &model, "Comments")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.commentFocus {
		t.Fatal("expected comment list focus")
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("opening edit composer should not submit work")
	}
	if next.mode != modeComment {
		t.Fatalf("mode = %v", next.mode)
	}
	if !next.commentEditing || next.commentEditID != "10002" || next.commentEditIssueKey != "ABC-1" {
		t.Fatalf("edit target = editing:%v issue:%q id:%q", next.commentEditing, next.commentEditIssueKey, next.commentEditID)
	}
	if next.commentDraft != "Second comment" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
	view := next.render()
	for _, want := range []string{"Edit Comment", "Second comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestCommentEditConfirmsUpdatesAndRefreshesComments(t *testing.T) {
	store := newFakeActiveViewStore()
	model := NewModel(&fakeIssueSearcher{
		updatedComment: jira.Comment{ID: "10001", Author: "Jane", Body: "Updated comment"},
		comments:       []jira.Comment{{ID: "10001", Author: "Jane", Body: "Updated comment"}},
	}, "project = ABC", WithActiveViewStore(store, "https://example.atlassian.net"))
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {{ID: "10001", Author: "Jane", Body: "Old comment"}}}
	model.commentEditing = true
	model.commentEditIssueKey = "ABC-1"
	model.commentEditID = "10001"
	model.commentDraft = "Updated comment"
	model.commentEditor = newCommentEditor("Updated comment")
	model.commentEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if !next.commentConfirm {
		t.Fatal("expected confirmation state")
	}
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected update comment command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)

	resultMsg := next.waitForWorkerResult()()
	updated, cmd = next.Update(resultMsg)
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode after update = %v", next.mode)
	}
	if !strings.Contains(next.detailNotice, "Comment updated") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if cmd == nil || !next.commentsLoading {
		t.Fatal("expected comments refresh after update")
	}
	if _, ok := next.comments["ABC-1"]; ok {
		t.Fatalf("expected stale comments cleared: %#v", next.comments["ABC-1"])
	}
	if store.deletedComments != "ABC-1" {
		t.Fatalf("deleted persistent comments = %q", store.deletedComments)
	}
}

func TestCommentComposerCancelReturnsToDetail(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.commentDraft = "Never mind"

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next := updated.(Model)

	if next.mode != modeDetail {
		t.Fatalf("mode = %v", next.mode)
	}
	if next.commentDraft != "" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerAcceptsSpaceNewlineAndCursorEditing(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "H", Code: 'H'}),
		tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}),
		tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace}),
		tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}),
		tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "N", Code: 'N'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}),
		tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "K", Code: 'K'}),
		tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Text: "!", Code: '!'}),
	} {
		updated, _ := model.Update(key)
		model = updated.(Model)
	}

	want := "Hi there\nNext\n!Kp"
	if model.commentDraft != want {
		t.Fatalf("commentDraft = %q, want %q", model.commentDraft, want)
	}
}

func TestCommentComposerAcceptsPaste(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.PasteMsg{Content: "first line\nsecond line"})
	next := updated.(Model)

	if next.commentDraft != "first line\nsecond line" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerFormattingControlsInsertTokens(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.commentEditor = newCommentEditor("")
	model.commentEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+b"}))
	model = updated.(Model)
	if model.commentDraft != "****" {
		t.Fatalf("bold draft = %q", model.commentDraft)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+e"}))
	model = updated.(Model)
	if model.commentDraft != "**__**" {
		t.Fatalf("italic draft = %q", model.commentDraft)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+g"}))
	model = updated.(Model)
	if model.commentDraft != "**_``_**" {
		t.Fatalf("code draft = %q", model.commentDraft)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+l"}))
	model = updated.(Model)
	if !strings.Contains(model.commentDraft, "\n- ") {
		t.Fatalf("bullet draft = %q", model.commentDraft)
	}
}

func TestCommentComposerTreatsQAsText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	next := updated.(Model)

	if next.mode != modeComment {
		t.Fatalf("mode = %v", next.mode)
	}
	if next.commentDraft != "q" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerTreatsGlobalActionKeysAsText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	for _, key := range []string{"r", "e", "q", "u", "i", "r", "e", "d", " ", "b", "y", "m", "l", "o", "c"} {
		msg := tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]})
		if key == " " {
			msg = tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace})
		}
		updated, _ := model.Update(msg)
		model = updated.(Model)
		if model.mode != modeComment {
			t.Fatalf("mode after %q = %v", key, model.mode)
		}
	}

	if model.commentDraft != "required bymloc" {
		t.Fatalf("commentDraft = %q", model.commentDraft)
	}
}

func TestCommentComposerTabReviewsDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.commentDraft = "Ready to post"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("tab review should not produce a command before confirmation")
	}
	if !next.commentConfirm {
		t.Fatal("expected confirmation state")
	}
	if next.commentEditor.Focused() {
		t.Fatal("expected editor to blur while reviewing")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next = updated.(Model)
	if next.commentConfirm {
		t.Fatal("expected confirmation state to close")
	}
	if !next.commentEditor.Focused() {
		t.Fatal("expected editor to refocus after returning to edit")
	}
}

func TestCommentComposerRendersEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 90
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	view := model.render()
	for _, want := range []string{"Add Comment", "──", "rite a Jira comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	model.commentDraft = "line one\n"
	view = model.render()
	if !strings.Contains(view, "line one") {
		t.Fatalf("expected draft in %q", view)
	}
}

func TestCommentComposerRendersNoticeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.detailNotice = "Write a comment before posting."

	view := model.render()

	for _, want := range []string{"Notice", "Write a comment before posting."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerSubmittingUsesSectionHeader(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentSubmitting = true

	view := model.render()

	for _, want := range []string{"Posting Comment", "──", "Posting comment..."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerIsBoundedAndScrollsDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 90
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
	}, "\n")

	view := model.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
	if !strings.Contains(view, "Lines 1-") {
		t.Fatalf("expected draft pagination indicator in %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	if next.commentEditor.ScrollYOffset() == 0 {
		t.Fatal("expected editor viewport to advance")
	}
	view = next.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines after scroll = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerShowsDetectedLinks(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "See https://example.test/run/1 and email ops@example.test"

	view := model.render()

	for _, want := range []string{"Detected Links", "──", "https://example.test/run/1", "ops@example.test"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerShowsDetectedLinksInReview(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "See https://example.test/run/1"

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	view := next.render()

	for _, want := range []string{"Review Comment", "Detected Links", "https://example.test/run/1", "Post this comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerShowsUnresolvedMentions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "Please check with @jane.doe"

	view := model.render()

	for _, want := range []string{"Unresolved Mentions", "──", "@jane.doe", "Use @ to select Jira users"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerMentionPickerSearchesAndInsertsUser(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("@ should open mention picker without submitting work")
	}
	if !next.mentionPickerOpen {
		t.Fatal("expected mention picker to open")
	}
	if activeKeyContext(next) != keyContextMentionPicker {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
	view := next.render()
	for _, want := range []string{"Mention User", "──", "type to search Jira", "users"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected mention picker text %q in %q", want, view)
		}
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "J", Code: 'J'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected user search command")
	}
	if next.mentionQuery != "J" {
		t.Fatalf("mentionQuery = %q", next.mentionQuery)
	}
	if !next.mentionSearchLoading {
		t.Fatal("expected user search loading state")
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.mentionSearchReqID,
		Kind: worker.KindSearchUsers,
		SearchUsers: &worker.SearchUsersResult{
			Query: "J",
			Users: []jira.User{
				{AccountID: "abc-123", DisplayName: "Jane Doe", Email: "jane@example.test", Active: true},
				{AccountID: "def-456", DisplayName: "John Doe", Email: "john@example.test", Active: true},
				{AccountID: "ghi-789", DisplayName: "Jill Doe", Email: "jill@example.test", Active: true},
			},
		},
	}})
	next = updated.(Model)
	if next.mentionSearchLoading {
		t.Fatal("expected user search loading state to clear")
	}
	view = next.render()
	for _, want := range []string{"Jane Doe", "John Doe", "Jill Doe", "Filter: J"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if !strings.Contains(view, "> Jane Doe") {
		t.Fatalf("expected first user to be visibly selected in %q", view)
	}
	if strings.Contains(view, "jane@example.test") {
		t.Fatalf("expected mention picker to hide email by default: %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	next = updated.(Model)
	if next.mentionCursor != 1 {
		t.Fatalf("mentionCursor = %d", next.mentionCursor)
	}
	view = next.render()
	if !strings.Contains(view, "> John Doe") {
		t.Fatalf("expected second user to be visibly selected in %q", view)
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.mentionPickerOpen {
		t.Fatal("expected mention picker to close")
	}
	if next.commentDraft != "@John Doe" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
	if !next.commentEditor.Focused() {
		t.Fatal("expected comment editor to regain focus")
	}
	if len(next.commentMentions) != 1 {
		t.Fatalf("commentMentions = %#v", next.commentMentions)
	}
	if next.commentMentions[0].AccountID != "def-456" || next.commentMentions[0].Text != "@John Doe" {
		t.Fatalf("commentMentions[0] = %#v", next.commentMentions[0])
	}
	view = next.render()
	if strings.Contains(view, "Unresolved Mentions") {
		t.Fatalf("selected mention should not render unresolved warning: %q", view)
	}
}

func TestCommentComposerMentionPickerCancelKeepsLiteralText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	next = updated.(Model)

	if next.mentionPickerOpen {
		t.Fatal("expected mention picker to close")
	}
	if next.commentDraft != "@ops" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerMentionPickerTreatsJAsFilterText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected user search command")
	}
	if next.mentionQuery != "j" {
		t.Fatalf("mentionQuery = %q", next.mentionQuery)
	}
}

func TestCommentComposerMentionPickerFilterUsesCursorAwareTextInput(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "J", Code: 'J'}),
		tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}),
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}

	if model.mentionQuery != "Jho" {
		t.Fatalf("mentionQuery = %q", model.mentionQuery)
	}
	if model.commentDraft != "" {
		t.Fatalf("commentDraft should not change while mention picker is open: %q", model.commentDraft)
	}
}
