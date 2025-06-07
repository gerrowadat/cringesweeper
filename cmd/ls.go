package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [username]",
	Short: "List recent posts from social media timelines",
	Long: `List and display posts from a user's social media timeline.

Supports multiple platforms including Bluesky and Mastodon. Shows post content,
timestamps, author information, and post types (original, repost, reply, etc.).

By default, shows recent posts (typically 10 most recent). Use --continue to
keep searching further back in time until no more posts are found. Use age
filters like --max-post-age or --before-date to limit results to specific
time periods.

The username can be provided as an argument or via environment variables.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		continueUntilEnd, _ := cmd.Flags().GetBool("continue")
		limitStr, _ := cmd.Flags().GetString("limit")
		maxAgeStr, _ := cmd.Flags().GetString("max-post-age")
		beforeDateStr, _ := cmd.Flags().GetString("before-date")

		// Get username with fallback priority: argument > saved credentials > environment
		argUsername := ""
		if len(args) > 0 {
			argUsername = args[0]
		}

		username, err := internal.GetUsernameForPlatform(platform, argUsername)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		client, exists := internal.GetClient(platform)
		if !exists {
			fmt.Printf("Error: Unsupported platform '%s'. Supported platforms: bluesky, mastodon\n", platform)
			os.Exit(1)
		}

		// Parse limit
		limit := 10 // default
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil {
				fmt.Printf("Error parsing limit: %v\n", err)
				os.Exit(1)
			}
			if parsedLimit <= 0 {
				fmt.Printf("Error: limit must be a positive number\n")
				os.Exit(1)
			}
			limit = parsedLimit
		}

		// Parse age filters
		var maxAge *time.Duration
		var beforeDate *time.Time

		if maxAgeStr != "" {
			duration, err := parseDuration(maxAgeStr)
			if err != nil {
				fmt.Printf("Error parsing max-post-age: %v\n", err)
				os.Exit(1)
			}
			maxAge = &duration
		}

		if beforeDateStr != "" {
			date, err := parseDate(beforeDateStr)
			if err != nil {
				fmt.Printf("Error parsing before-date: %v\n", err)
				os.Exit(1)
			}
			beforeDate = &date
		}

		// Perform listing
		if continueUntilEnd {
			performContinuousListing(client, username, limit, maxAge, beforeDate)
		} else {
			performSingleListing(client, username, limit, maxAge, beforeDate)
		}
	},
}

func performSingleListing(client internal.SocialClient, username string, limit int, maxAge *time.Duration, beforeDate *time.Time) {
	posts, err := client.FetchUserPosts(username, limit)
	if err != nil {
		fmt.Printf("Error fetching posts from %s: %v\n", client.GetPlatformName(), err)
		os.Exit(1)
	}

	// Filter posts by age criteria if specified
	filteredPosts := filterPostsByAge(posts, maxAge, beforeDate)
	
	if len(filteredPosts) == 0 {
		if maxAge != nil || beforeDate != nil {
			fmt.Println("No posts match the specified age criteria")
		} else {
			fmt.Println("No posts found")
		}
		return
	}

	fmt.Printf("Posts from %s", client.GetPlatformName())
	if maxAge != nil || beforeDate != nil {
		fmt.Printf(" (filtered by age criteria)")
	}
	fmt.Printf(":\n\n")

	displayPostsStreaming(filteredPosts)
}

func performContinuousListing(client internal.SocialClient, username string, batchLimit int, maxAge *time.Duration, beforeDate *time.Time) {
	platform := client.GetPlatformName()
	round := 1
	totalDisplayed := 0
	headerShown := false

	fmt.Printf("Searching %s for posts", platform)
	if maxAge != nil || beforeDate != nil {
		fmt.Printf(" matching age criteria")
	}
	fmt.Printf(" (will continue until no more posts found)...\n\n")

	for {
		posts, err := client.FetchUserPosts(username, batchLimit)
		if err != nil {
			fmt.Printf("Error in round %d: %v\n", round, err)
			break
		}

		// Filter posts by age criteria if specified
		filteredPosts := filterPostsByAge(posts, maxAge, beforeDate)

		if len(filteredPosts) == 0 {
			if round == 1 {
				fmt.Println("No posts match the specified criteria")
			} else {
				fmt.Printf("\nNo more posts found. Search complete after %d rounds.\n", round)
				fmt.Printf("Total posts displayed: %d\n", totalDisplayed)
			}
			break
		}

		// Show header on first batch with results
		if !headerShown {
			fmt.Printf("Posts from %s:\n\n", platform)
			headerShown = true
		}

		// Stream the posts immediately
		for _, post := range filteredPosts {
			displaySinglePost(post, totalDisplayed+1)
			totalDisplayed++
		}

		round++

		// Small delay between rounds to be respectful to APIs
		time.Sleep(time.Second)
	}
}

func filterPostsByAge(posts []internal.Post, maxAge *time.Duration, beforeDate *time.Time) []internal.Post {
	if maxAge == nil && beforeDate == nil {
		return posts
	}

	var filtered []internal.Post
	now := time.Now()

	for _, post := range posts {
		shouldInclude := true

		// Check max age criteria
		if maxAge != nil {
			if now.Sub(post.CreatedAt) > *maxAge {
				shouldInclude = false
			}
		}

		// Check before date criteria
		if beforeDate != nil {
			if !post.CreatedAt.Before(*beforeDate) {
				shouldInclude = false
			}
		}

		if shouldInclude {
			filtered = append(filtered, post)
		}
	}

	return filtered
}

func displayPostsStreaming(posts []internal.Post) {
	for i, post := range posts {
		displaySinglePost(post, i+1)
	}
}

func displaySinglePost(post internal.Post, index int) {
	fmt.Printf("Post %d", index)

	// Show post type indicator
	switch post.Type {
	case internal.PostTypeRepost:
		fmt.Printf(" [REPOST]")
	case internal.PostTypeReply:
		fmt.Printf(" [REPLY]")
	case internal.PostTypeQuote:
		fmt.Printf(" [QUOTE]")
	case internal.PostTypeLike:
		fmt.Printf(" [LIKE]")
	}
	fmt.Printf(":\n")

	fmt.Printf("  Author: @%s", post.Handle)
	if post.Author != "" && post.Author != post.Handle {
		fmt.Printf(" (%s)", post.Author)
	}
	fmt.Printf("\n")

	fmt.Printf("  Posted: %s\n", post.CreatedAt.Format("2006-01-02 15:04:05"))

	// Handle reposts specially
	if post.Type == internal.PostTypeRepost && post.OriginalPost != nil {
		fmt.Printf("  Reposted from: @%s", post.OriginalHandle)
		if post.OriginalAuthor != "" && post.OriginalAuthor != post.OriginalHandle {
			fmt.Printf(" (%s)", post.OriginalAuthor)
		}
		fmt.Printf("\n")
		fmt.Printf("  Original content: %s\n", post.OriginalPost.Content)
	} else {
		fmt.Printf("  Content: %s\n", post.Content)
	}

	// Show engagement metrics if available
	if post.LikeCount > 0 || post.RepostCount > 0 || post.ReplyCount > 0 {
		fmt.Printf("  Engagement: ")
		var metrics []string
		if post.LikeCount > 0 {
			metrics = append(metrics, fmt.Sprintf("%d likes", post.LikeCount))
		}
		if post.RepostCount > 0 {
			metrics = append(metrics, fmt.Sprintf("%d reposts", post.RepostCount))
		}
		if post.ReplyCount > 0 {
			metrics = append(metrics, fmt.Sprintf("%d replies", post.ReplyCount))
		}
		fmt.Printf("%s\n", fmt.Sprintf("%v", metrics))
	}

	if post.URL != "" {
		fmt.Printf("  URL: %s\n", post.URL)
	}
	fmt.Println()
}

func displayPosts(posts []internal.Post, platform string) {
	if len(posts) == 0 {
		fmt.Println("No posts found")
		return
	}

	fmt.Printf("Recent posts from %s:\n\n", platform)

	for i, post := range posts {
		fmt.Printf("Post %d", i+1)

		// Show post type indicator
		switch post.Type {
		case internal.PostTypeRepost:
			fmt.Printf(" [REPOST]")
		case internal.PostTypeReply:
			fmt.Printf(" [REPLY]")
		case internal.PostTypeQuote:
			fmt.Printf(" [QUOTE]")
		case internal.PostTypeLike:
			fmt.Printf(" [LIKE]")
		}
		fmt.Printf(":\n")

		fmt.Printf("  Author: @%s", post.Handle)
		if post.Author != "" && post.Author != post.Handle {
			fmt.Printf(" (%s)", post.Author)
		}
		fmt.Printf("\n")

		fmt.Printf("  Posted: %s\n", post.CreatedAt.Format("2006-01-02 15:04:05"))

		// Handle reposts specially
		if post.Type == internal.PostTypeRepost && post.OriginalPost != nil {
			fmt.Printf("  Reposted from: @%s", post.OriginalHandle)
			if post.OriginalAuthor != "" && post.OriginalAuthor != post.OriginalHandle {
				fmt.Printf(" (%s)", post.OriginalAuthor)
			}
			fmt.Printf("\n")
			fmt.Printf("  Original content: %s\n", post.OriginalPost.Content)
		} else {
			fmt.Printf("  Content: %s\n", post.Content)
		}

		// Show engagement metrics if available
		if post.LikeCount > 0 || post.RepostCount > 0 || post.ReplyCount > 0 {
			fmt.Printf("  Engagement: ")
			var metrics []string
			if post.LikeCount > 0 {
				metrics = append(metrics, fmt.Sprintf("%d likes", post.LikeCount))
			}
			if post.RepostCount > 0 {
				metrics = append(metrics, fmt.Sprintf("%d reposts", post.RepostCount))
			}
			if post.ReplyCount > 0 {
				metrics = append(metrics, fmt.Sprintf("%d replies", post.ReplyCount))
			}
			fmt.Printf("%s\n", fmt.Sprintf("%v", metrics))
		}

		if post.URL != "" {
			fmt.Printf("  URL: %s\n", post.URL)
		}
		fmt.Println()
	}
}

func init() {
	rootCmd.AddCommand(lsCmd)
	lsCmd.Flags().StringP("platform", "p", "bluesky", "Social media platform (bluesky, mastodon)")
	lsCmd.Flags().String("limit", "10", "Maximum number of posts to fetch per batch")
	lsCmd.Flags().String("max-post-age", "", "Only show posts older than this (e.g., 30d, 1y, 24h)")
	lsCmd.Flags().String("before-date", "", "Only show posts created before this date (YYYY-MM-DD or MM/DD/YYYY)")
	lsCmd.Flags().Bool("continue", false, "Continue searching and fetching posts until no more are found")
}
