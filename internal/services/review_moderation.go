package services

import (
	"strings"
	"time"
)

// bannedTerms is a minimal, hardcoded starting point for catching obvious
// profanity and spam (contact-info fishing, drive-by-link spam) at
// submission time. It is intentionally small — swap in a proper
// profanity/spam-detection service if false negatives become a problem.
var bannedTerms = []string{
	// Profanity (minimal set)
	"fuck", "shit", "bitch", "asshole", "bastard", "cunt",
	// Spam / contact-info fishing patterns
	"http://", "https://", "www.", "buy now at", "click here",
	"whatsapp me", "call me at", "dm me", "contact me at",
}

// isFlaggedContent does a case-insensitive substring match against
// bannedTerms across the review's title and comment.
func isFlaggedContent(title, comment string) bool {
	text := strings.ToLower(title + " " + comment)
	for _, term := range bannedTerms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

const (
	// reportThreshold is how many distinct users must report a review
	// before it's pulled from public view and sent back to the admin
	// moderation queue.
	reportThreshold = 3

	// fraudVelocityWindow / fraudVelocityLimit: a user posting this many
	// reviews within this window looks bot-like, so new reviews from them
	// are held for manual approval instead of auto-approved.
	fraudVelocityWindow = 10 * time.Minute
	fraudVelocityLimit  = 3
)
