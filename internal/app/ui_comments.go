package app

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
)

// commentState is the comments of the song or episode Now Playing shows,
// loaded a page at a time as they scroll, for a source that has them.
type commentState struct {
	track   string // whose comments they are: its source and ID
	lib     *library
	id      string // its ID
	list    []music.Comment
	total   int
	more    bool
	loaded  bool
	loading bool
	err     string
	scroll  ui.ScrollState
	draft   string // what the listener is writing
	posting bool
}

// commentsFor makes the comments those of tr, loading their first page
// once it changed: Now Playing does it as it shows, for the count on the
// pane's tab.
func (a *app) commentsFor(tr *music.Track) *commentState {
	cs := &a.comments
	if key := tr.Source + "\x00" + tr.ID; cs.track != key {
		*cs = commentState{track: key, lib: a.trackLib(tr), id: tr.ID}
		if cs.lib.can("comments") {
			a.loadComments()
		}
	}
	return cs
}

// loadComments loads the next page of comments.
func (a *app) loadComments() {
	cs := &a.comments
	if cs.loading || cs.lib == nil {
		return
	}
	lib, track, id, offset := cs.lib, cs.track, cs.id, len(cs.list)
	cs.loading, cs.err = true, ""
	go func() {
		page, err := answered(lib.src.Comments(id, offset))
		a.post(func() {
			if cs.track != track {
				return
			}
			cs.loading = false
			if err != nil {
				cs.err = a.problem(lib, err, T("Couldn't load the comments"))
				return
			}
			cs.loaded = true
			// New comments shift the pages: those already shown come again.
			seen := make(map[string]bool, len(cs.list))
			for _, cm := range cs.list {
				seen[cm.ID] = true
			}
			for _, cm := range page.Comments {
				if !seen[cm.ID] {
					cs.list = append(cs.list, cm)
				}
			}
			cs.total, cs.more = page.Total, page.More && len(page.Comments) > 0
			if cs.total < len(cs.list) {
				cs.total = len(cs.list)
			}
		})
	}()
}

// topComments are the comments most liked, for the quiet parts of a song.
func (cs *commentState) topComments() []*music.Comment {
	var top []*music.Comment
	for i := range cs.list {
		if cs.list[i].Likes > 0 {
			top = append(top, &cs.list[i])
		}
	}
	slices.SortStableFunc(top, func(x, y *music.Comment) int { return y.Likes - x.Likes })
	return top[:min(len(top), 10)]
}

// commentCount is the count the comments' tab shows: the last song's
// while the next one's loads, so that it rolls from one to the other.
func (a *app) commentCount() int {
	switch cs := &a.comments; {
	case cs.loaded:
		return cs.total
	case cs.err != "":
		return 0
	}
	return a.tabCount.n
}

// rollState is a count as it rolls from one value to another.
type rollState struct {
	n    int
	up   bool       // whether it went up, last it changed
	exit *ui.Motion // where the characters shown go as it changes
}

// rollLine is how far a character rolls, about a line of the tab's text.
const rollLine = 16

// rollCount shows n as text, each character in a slot of its own, as
// the wheels of a counter: one that changes rolls out as the next rolls
// in, upward as n grows and downward as it shrinks, the others staying.
func rollCount(c *ui.Context, st *rollState, n int, text string, fg ui.Color) {
	if n != st.n {
		st.up = n > st.n
		dir := float32(1)
		if !st.up {
			dir = -1
		}
		if st.exit != nil {
			// The characters shown leave from this frame on, which first
			// knows the way.
			st.exit.Y = -dir * rollLine
		}
		st.n, st.exit = n, &ui.Motion{}
	}
	dir := float32(1)
	if !st.up {
		dir = -1
	}
	chars := []rune(text)
	// Clipped to its line, a character leaves out of sight.
	ui.Row(c).ClipY().Children(func() {
		for i, ch := range chars {
			// Slots go by their place from the right, as the wheels of a
			// counter do: the units are always the last.
			k := len(chars) - 1 - i
			roll := ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.15}, Delay: time.Duration(k) * 30 * time.Millisecond}
			slot := ui.Box(c.Key(k)).Transition(ui.ElementTransition{
				Animation: &roll, Size: true,
				Enter: &ui.Motion{Y: dir * rollLine, Collapse: true},
				Exit:  &ui.Motion{Collapse: true},
			})
			slot.Children(func() {
				ui.Text(c.Key(ch), string(ch)).FontSize(13).FontWeight(600).FontFeatures("tnum").TextColor(fg).
					Transition(ui.ElementTransition{Animation: &roll, Colors: true, Enter: &ui.Motion{Y: dir * rollLine}, Exit: st.exit})
			})
		}
	})
}

// count formats a count shortly, as people read it in the listener's language.
func count(n int) string { return i18n.Count(n) }

// commentsPane lists the comments of tr, which load on as they scroll into
// view, under a place to write one, if the source takes them.
func (a *app) commentsPane(c *ui.Context, tr *music.Track) {
	cs := a.commentsFor(tr)
	if cs.lib.can("postComment") {
		a.commentBox(c, cs)
	}
	if len(cs.list) == 0 {
		if cs.loading || !cs.loaded && cs.err == "" {
			ui.Column(c).Grow(1).Padding(4, 8, 0, 0).Gap(2).Children(func() {
				s := npShimmer(c)
				s.text(c, 13, 1.3).Width(56).Margin(18, 12, 6, 12)
				commentSkeleton(c, s, 6)
			})
			return
		}
		ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
			switch {
			case cs.err != "":
				ui.Text(c, cs.err).FontSize(15).TextColor(white.Alpha(0.5))
				if pillButton(c.Key("retry"), iconComment, T("Retry"), white.Alpha(0.14), white).Clicked() {
					a.loadComments()
				}
			default:
				ui.Icon(c, iconComment).Size(40, 40).TextColor(white.Alpha(0.3))
				ui.Text(c, T("No comments yet")).FontSize(17).FontWeight(600).TextColor(white.Alpha(0.45))
			}
		})
		return
	}
	now := c.Now()
	scroll := ui.Scroll(c.Key("comments")).Grow(1).TrackScroll(&cs.scroll).Padding(4, 8, 48, 0)
	var end ui.Rect
	scroll.Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for i := range cs.list {
				a.commentRow(c.Key(cs.list[i].ID), &cs.list[i], now)
			}
			if cs.loading {
				commentSkeleton(c.Key("more"), npShimmer(c), 2)
			}
			ui.Row(c.Key("foot")).Height(56).Center().Children(func() {
				switch {
				case cs.loading:
				case cs.err != "":
					retry := ui.ButtonBase(c.Key("retry")).Padding(6, 14).Radius(999).Background(white.Alpha(0.1)).Label(T("Retry"))
					retry.Children(func() {
						ui.Text(c, T("Couldn't load. Click to try again.")).FontSize(12.5).TextColor(white.Alpha(0.7))
					})
					if retry.Clicked() {
						a.loadComments()
					}
				case !cs.more:
					ui.Text(c, T("That's all the comments")).FontSize(12).TextColor(white.Alpha(0.35))
				}
			})
			end = ui.Box(c.Key("end")).Height(0).Bounds()
		})
	})
	// The next page, once the foot is within a screen of coming into view.
	if box := scroll.Bounds(); cs.more && !cs.loading && cs.err == "" && end.W > 0 && end.Y < box.Y+box.H*2 {
		a.loadComments()
	}
}

// commentBox is where the listener writes a comment.
func (a *app) commentBox(c *ui.Context, cs *commentState) {
	ui.Row(c.Key("comment-box")).Gap(8).AlignItems(ui.Center).Margin(0, 8, 8, 0).Children(func() {
		in := ui.TextInput(c.Key("draft"), &cs.draft).Placeholder(T("Write a comment…")).Label(T("Write a comment…")).FontSize(13.5).Grow(1).Shrink(1).MinWidth(0)
		send := pillButton(c.Key("send"), nil, T("Post"), white.Alpha(0.14), white)
		if strings.TrimSpace(cs.draft) == "" || cs.posting {
			send.Disabled(true).Opacity(0.5)
		}
		if send.Clicked() || in.Submitted() {
			a.postComment(cs)
		}
	})
}

// postComment posts what the listener wrote, showing it first of the comments
// once the source took it.
func (a *app) postComment(cs *commentState) {
	text := strings.TrimSpace(cs.draft)
	lib, track, id := cs.lib, cs.track, cs.id
	if text == "" || cs.posting || lib == nil {
		return
	}
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	cs.posting = true
	go func() {
		cm, err := lib.src.PostComment(id, text)
		a.post(func() {
			if cs.track != track {
				return
			}
			cs.posting = false
			if err != nil {
				a.actionFailed(lib, err, T("Couldn't post your comment. Try again later."))
				return
			}
			cs.draft = ""
			if cm != nil {
				cs.list = append([]music.Comment{*cm}, cs.list...)
				cs.total++
			}
		})
	}()
}

// commentRow is a comment: who wrote it, what, when, and how many liked it,
// with a thumb to like it too.
func (a *app) commentRow(c *ui.Context, cm *music.Comment, now time.Time) {
	ui.Row(c).Padding(12).Gap(12).Radius(12).AlignItems(ui.Start).Children(func() {
		a.avatar(c, cm.AvatarURL, 34, white.Alpha(0.1))
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, cm.Author).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.6)).SingleLine().Shrink(1).MinWidth(0)
				ui.Spacer(c)
				a.commentLike(c.Key("like"), cm)
			})
			ui.Text(c, cm.Text).FontSize(15).LineHeight(1.55).TextColor(white.Alpha(0.92))
			if cm.Time > 0 {
				ui.Text(c, i18n.Ago(time.UnixMilli(cm.Time), now)).FontSize(11.5).TextColor(white.Alpha(0.38))
			}
		})
	})
}

// commentLike is a comment's thumb, with how many liked it, which likes it
// or unlikes it, if the source lets it: in red once liked.
func (a *app) commentLike(c *ui.Context, cm *music.Comment) {
	can := a.comments.lib.can("likeComment")
	if !can && cm.Likes == 0 {
		return
	}
	label, fg := T("Like"), white.Alpha(0.42)
	if cm.Liked {
		label, fg = T("Unlike"), a.pal.accent
	}
	b := ui.ButtonBase(c).Gap(4).Padding(3, 6).Margin(-3, -6).Radius(999).AlignItems(ui.Center).FocusRing(false).Label(label).Disabled(!can)
	if b.Hovered() && !cm.Liked && can {
		fg = white.Alpha(0.75)
	}
	b.TextColor(b.AnimateColor("fg", fg, ui.Animation{Duration: 120 * time.Millisecond}))
	b.Children(func() {
		icon := iconThumb
		if cm.Liked {
			icon = iconThumbFill
		}
		ui.Icon(c, icon).Size(13, 13)
		if cm.Likes > 0 {
			ui.Text(c, count(cm.Likes)).FontSize(12).FontFeatures("tnum")
		}
	})
	if b.Clicked() {
		a.toggleCommentLike(cm.ID)
	}
}

// toggleCommentLike likes a comment, or unlikes it, at once in the app and
// then with the source, which may refuse: it goes back then. Signed out, it
// asks the listener to sign in.
func (a *app) toggleCommentLike(id string) {
	cs := &a.comments
	lib := cs.lib
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	track, trackID, like := cs.track, cs.id, true
	if cm := cs.find(id); cm != nil {
		like = !cm.Liked
	}
	cs.setLiked(id, like)
	go func() {
		err := lib.src.LikeComment(trackID, id, like)
		a.post(func() {
			if err != nil && cs.track == track {
				cs.setLiked(id, !like)
				a.actionFailed(lib, err, T("That didn't work. Try again later."))
			}
		})
	}()
}

// find returns the comment id, or nil.
func (cs *commentState) find(id string) *music.Comment {
	for i := range cs.list {
		if cs.list[i].ID == id {
			return &cs.list[i]
		}
	}
	return nil
}

// setLiked marks the comment id liked, or not, counting the listener in or
// out.
func (cs *commentState) setLiked(id string, like bool) {
	if cm := cs.find(id); cm != nil && cm.Liked != like {
		cm.Liked = like
		if like {
			cm.Likes++
		} else {
			cm.Likes = max(cm.Likes-1, 0)
		}
	}
}
