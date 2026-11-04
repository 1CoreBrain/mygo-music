package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

// commentsApp is Now Playing showing the comments of a song, one of them
// loaded, and what the app posts, its hearts out of the way of the thumbs.
func commentsApp() (*app, *ui.Tester, chan func()) {
	a := newTestApp()
	ch := posts(a)
	without(a, "like", "isLiked")
	a.account = &music.Account{ID: "1"}
	a.pl.Cue([]music.Track{song("2", "二")})
	a.pl.SetLyrics(nil)
	a.comments = commentState{track: "\x002", lib: a.library, id: "2", loaded: true, total: 1, list: []music.Comment{{ID: "9", Text: "好听", Likes: 41}}}
	a.nowPlaying, a.npPane = true, paneComments
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	return a, tt, ch
}

// The count on the comments' tab holds the last song's while the next
// one's loads, for it to roll from one to the other, and goes once the
// next song turns out to have none.
func TestCommentCountHoldsWhileLoading(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.comments = commentState{track: "\x002", lib: a.library, id: "2", loaded: true, total: 128}
	tt.Frame()
	a.pl.Next()
	a.comments = commentState{track: "\x003", lib: a.library, id: "3", loading: true}
	tt.Frame()
	if a.tabCount.n != 128 {
		t.Fatalf("the count went to %d as the next song's loads", a.tabCount.n)
	}
	a.comments.loading, a.comments.loaded, a.comments.total = false, true, 96
	tt.Frame()
	if a.tabCount.n != 96 {
		t.Errorf("the count is %d once loaded", a.tabCount.n)
	}
	a.pl.Next()
	a.comments = commentState{track: "\x001", lib: a.library, id: "1", loaded: true}
	tt.Advance(time.Second)
	tt.Frame()
	if a.tabCount.n != 0 {
		t.Errorf("a song without comments counts %d", a.tabCount.n)
	}
}

// Now Playing shows an episode's comments under a tab of their own beside
// what it is about, which stays chosen as the frames go.
func TestEpisodeComments(t *testing.T) {
	asked := make(chan string, 4)
	a := newTestApp()
	fake(a).comments = func(track string, offset int) (*music.CommentPage, error) {
		asked <- track
		return &music.CommentPage{}, nil
	}
	a.pl.Cue([]music.Track{{ID: "13", Name: "第一期", ShowID: "5", Duration: 600000}})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if track := wait(t, asked, "the comments"); track != "13" {
		t.Fatalf("asked for the comments of %s", track)
	}
	if a.npPane != paneShow {
		t.Fatalf("opened on pane %d", a.npPane)
	}
	if err := tt.Click("Comments"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.npPane != paneComments {
		t.Errorf("on pane %d", a.npPane)
	}
}

// A comment's thumb likes it at once, counting the listener in, and asks the
// source to; again, it unlikes it.
func TestLikeComment(t *testing.T) {
	type call struct {
		track, id string
		like      bool
	}
	calls := make(chan call, 4)
	a, tt, _ := commentsApp()
	fake(a).likeComment = func(track, id string, like bool) error { calls <- call{track, id, like}; return nil }
	if err := tt.Click("Like"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the like"); c != (call{"2", "9", true}) {
		t.Errorf("asked the source for %+v", c)
	}
	if cm := a.comments.list[0]; !cm.Liked || cm.Likes != 42 {
		t.Fatalf("liked %v by %d", cm.Liked, cm.Likes)
	}
	tt.Frame()
	if err := tt.Click("Unlike"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the unlike"); c.like {
		t.Error("liked again")
	}
	if cm := a.comments.list[0]; cm.Liked || cm.Likes != 41 {
		t.Errorf("unliked: liked %v by %d", cm.Liked, cm.Likes)
	}
}

// A like the source refuses goes back, and says so.
func TestLikeCommentGoesBack(t *testing.T) {
	a, tt, ch := commentsApp()
	fake(a).likeComment = func(string, string, bool) error { return errors.New("offline") }
	if err := tt.Click("Like"); err != nil {
		t.Fatal(err)
	}
	if !a.comments.list[0].Liked {
		t.Fatal("not liked at once")
	}
	runPost(t, ch)
	if cm := a.comments.list[0]; cm.Liked || cm.Likes != 41 || a.toast == "" {
		t.Errorf("liked %v by %d, toast %q", cm.Liked, cm.Likes, a.toast)
	}
}

// Enter posts what the listener wrote, shown first of the comments once the
// source took it, emptying the field; what it refuses stays there to send
// again.
func TestPostComment(t *testing.T) {
	type call struct{ track, text string }
	calls := make(chan call, 4)
	var answer error
	a, tt, ch := commentsApp()
	fake(a).postComment = func(track, text string) (*music.Comment, error) {
		calls <- call{track, text}
		if answer != nil {
			return nil, answer
		}
		return &music.Comment{ID: "10", Text: text}, nil
	}
	if err := tt.Click("Write a comment…"); err != nil {
		t.Fatal(err)
	}

	answer = errors.New("offline")
	tt.Type("  真好听  ")
	tt.Key(0, ui.KeyEnter)
	if c := wait(t, calls, "the comment"); c != (call{"2", "真好听"}) {
		t.Fatalf("posted %+v", c)
	}
	runPost(t, ch)
	if len(a.comments.list) != 1 || a.comments.draft == "" || a.toast == "" {
		t.Fatalf("refused: listed %+v, draft %q, toast %q", a.comments.list, a.comments.draft, a.toast)
	}
	tt.Frame()

	answer = nil
	tt.Key(0, ui.KeyEnter)
	wait(t, calls, "the comment again")
	runPost(t, ch)
	tt.Frame()
	if len(a.comments.list) != 2 || a.comments.list[0].ID != "10" || a.comments.total != 2 || a.comments.draft != "" {
		t.Fatalf("listed %+v of %d, draft %q", a.comments.list, a.comments.total, a.comments.draft)
	}
}

// Posting signed out asks the listener to sign in, keeping what they wrote.
func TestPostCommentSignedOut(t *testing.T) {
	a, tt, _ := commentsApp()
	a.account = nil
	fake(a).postComment = func(string, string) (*music.Comment, error) {
		t.Error("asked the source")
		return nil, nil
	}
	if err := tt.Click("Write a comment…"); err != nil {
		t.Fatal(err)
	}
	tt.Type("真好听")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !a.signIn.open || a.comments.draft != "真好听" {
		t.Errorf("dialog open %v, draft %q", a.signIn.open, a.comments.draft)
	}
}

// Comments load on from where they ended, once their foot comes into view,
// and what a page shifted by new comments repeats isn't listed twice.
func TestCommentPagesOverlap(t *testing.T) {
	asked := make(chan int, 4)
	a := newTestApp()
	ch := posts(a)
	without(a, "like", "isLiked", "postComment")
	fake(a).comments = func(_ string, offset int) (*music.CommentPage, error) {
		asked <- offset
		if offset == 0 {
			return &music.CommentPage{Comments: []music.Comment{{ID: "1", Text: "一"}, {ID: "2", Text: "二"}}, More: true, Total: 3}, nil
		}
		return &music.CommentPage{Comments: []music.Comment{{ID: "2", Text: "二"}, {ID: "3", Text: "三"}}, Total: 3}, nil
	}
	a.pl.Cue([]music.Track{song("7", "宝船")})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneComments
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if got := wait(t, asked, "the first page"); got != 0 {
		t.Fatalf("asked from %d", got)
	}
	runPost(t, ch)
	settled(tt)
	if got := wait(t, asked, "the next page"); got != 2 {
		t.Fatalf("asked from %d", got)
	}
	runPost(t, ch)
	settled(tt)
	var ids []string
	for _, cm := range a.comments.list {
		ids = append(ids, cm.ID)
	}
	if len(ids) != 3 || ids[0] != "1" || ids[1] != "2" || ids[2] != "3" || len(asked) > 0 {
		t.Errorf("listed %v, %d more asked", ids, len(asked))
	}
}
