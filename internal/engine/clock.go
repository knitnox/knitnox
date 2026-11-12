package engine

import (
	"fmt"
	"time"
)

// Telling the AI what day and time it is.
//
// Models have no clock. In a chat, every user message sent to the model
// starts with a line saying when it was written:
//
//	[Wed, 11 Nov 2026, 19:05]
//	what is on my plan for tomorrow?
//
// The newest line is "now". Nothing about the time is put in the system
// prompt, and the stamp of an old message never changes, so the start of the
// prompt stays byte-for-byte the same from turn to turn. That is what lets
// providers reuse their prompt cache (cheaper and faster); a clock in the
// system prompt would throw the whole cache away every minute.
//
// Whose clock: in a person's own chat, their time zone (users.timezone, kept
// up to date by their browser; UTC while unknown). In a group chat people may
// be in different zones, so the server's own time is used.
//
// Agent runs start fresh every time, so there is no cache to protect: their
// prompt simply states the owner's date and time (see AgentSystemPrompt).

const stampLayout = "Mon, 2 Jan 2006, 15:04"

// timeStamp is the line put in front of a user message.
func timeStamp(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return "[" + t.In(loc).Format(stampLayout) + "]"
}

// messageStamp is the stamp for a stored message; createdAt is the database's
// UTC text form. An unreadable time gives no stamp.
func messageStamp(createdAt string, loc *time.Location) string {
	t, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return ""
	}
	return timeStamp(t, loc)
}

// zoneWords names the clock the stamps use, for the system prompt.
func zoneWords(loc *time.Location, serverTime bool) string {
	if serverTime {
		if loc == nil { // the context meter passes no zone; time.In(nil) panics
			loc = time.UTC
		}
		return "the server's time zone (UTC" + time.Now().In(loc).Format("-07:00") + ")"
	}
	if loc == nil || loc == time.UTC || loc.String() == "UTC" {
		return "UTC"
	}
	return "the user's time zone (" + loc.String() + ")"
}

// clockGuide is the fixed paragraph of the system prompt that explains the stamps.
func clockGuide(loc *time.Location, serverTime bool) string {
	return fmt.Sprintf("Every user message starts with a line in square brackets: the date and time it was sent, in %s. "+
		"The newest of these lines is the current date and time. Use it for anything about dates and times, "+
		"such as \"today\", \"tomorrow\" or \"in two hours\". Never write such a line yourself.", zoneWords(loc, serverTime))
}

// clockLine is "Wednesday, 11 November 2026, 19:05 (Asia/Dhaka, UTC+06:00)", for agent prompts.
func clockLine(now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	t := now.In(loc)
	zone := "UTC"
	if name := loc.String(); name != "UTC" {
		zone = name + ", UTC" + t.Format("-07:00")
	}
	return fmt.Sprintf("%s (%s)", t.Format("Monday, 2 January 2006, 15:04"), zone)
}
