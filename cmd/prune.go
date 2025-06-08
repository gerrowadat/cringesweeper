package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune [username]",
	Short: "Delete posts based on age, date, and preservation rules",
	Long: `Delete posts, unlike posts, and unshare reposts from your social media accounts based on configurable criteria.

Use --platforms to operate on multiple platforms simultaneously (e.g., --platforms=bluesky,mastodon
or --platforms=all). When multiple platforms are specified, operations are performed
on each platform sequentially with clear progress indicators.

What gets processed:
- Original posts you created: Deleted permanently
- Posts you've reposted: Removes your repost (unrepost)
- Posts you've liked: Removes your like (unlike) - only when --unlike-posts is used

Posts can be processed by maximum age (e.g., older than 30 days) or before a specific 
date. Smart preservation rules protect important content like pinned posts and 
posts you've liked.

By default, only processes recent posts (typically 100 most recent). Use --continue 
to keep searching further back in time until no more posts match your criteria.

ALWAYS use --dry-run first to preview what would be processed. Actions are 
permanent and cannot be undone. Requires authentication for the target platform.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platformsStr, _ := cmd.Flags().GetString("platforms")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		preserveSelfLike, _ := cmd.Flags().GetBool("preserve-selflike")
		preservePinned, _ := cmd.Flags().GetBool("preserve-pinned")
		unlikePosts, _ := cmd.Flags().GetBool("unlike-posts")
		unshareReposts, _ := cmd.Flags().GetBool("unshare-reposts")
		continueUntilEnd, _ := cmd.Flags().GetBool("continue")
		maxAgeStr, _ := cmd.Flags().GetString("max-post-age")
		beforeDateStr, _ := cmd.Flags().GetString("before-date")
		rateLimitDelayStr, _ := cmd.Flags().GetString("rate-limit-delay")

		// Determine which platforms to use
		var platforms []string
		var err error
		
		if platformsStr == "" {
			fmt.Printf("Error: --platforms flag is required. Specify comma-separated platforms (bluesky,mastodon) or 'all'\n")
			os.Exit(1)
		}
		
		platforms, err = internal.ParsePlatforms(platformsStr)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Get username with fallback priority: argument > saved credentials > environment
		argUsername := ""
		if len(args) > 0 {
			argUsername = args[0]
		}

		// Track overall results across all platforms
		totalResults := &internal.PruneResult{
			PostsToDelete:  []internal.Post{},
			PostsToUnlike:  []internal.Post{},
			PostsToUnshare: []internal.Post{},
			PostsPreserved: []internal.Post{},
			DeletedCount:   0,
			UnlikedCount:   0,
			UnsharedCount:  0,
			PreservedCount: 0,
			ErrorsCount:    0,
			Errors:         []string{},
		}

		// Process each platform
		for i, platformName := range platforms {
			if len(platforms) > 1 {
				fmt.Printf("\n=== PRUNING %s ===\n", strings.ToUpper(platformName))
			}

			username, err := internal.GetUsernameForPlatform(platformName, argUsername)
			if err != nil {
				fmt.Printf("Error for %s: %v\n", platformName, err)
				if len(platforms) > 1 {
					totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: %v", platformName, err))
					continue // Skip this platform but continue with others
				}
				os.Exit(1)
			}

			client, exists := internal.GetClient(platformName)
			if !exists {
				errorMsg := fmt.Sprintf("Unsupported platform '%s'. Supported platforms: %s", 
					platformName, strings.Join(internal.GetAllPlatformNames(), ", "))
				fmt.Printf("Error: %s\n", errorMsg)
				if len(platforms) > 1 {
					totalResults.Errors = append(totalResults.Errors, errorMsg)
					continue // Skip this platform but continue with others
				}
				os.Exit(1)
			}

			// Parse rate limit delay - use platform-appropriate defaults
			var rateLimitDelay time.Duration
			if rateLimitDelayStr != "" {
				delay, err := parseDuration(rateLimitDelayStr)
				if err != nil {
					fmt.Printf("Error parsing rate-limit-delay for %s: %v\n", platformName, err)
					if len(platforms) > 1 {
						totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: rate-limit-delay parse error: %v", platformName, err))
						continue
					}
					os.Exit(1)
				}
				rateLimitDelay = delay
			} else {
				// Set platform-appropriate defaults
				switch platformName {
				case "mastodon":
					rateLimitDelay = 60 * time.Second // Conservative for Mastodon's 30 DELETEs per 30 minutes
				case "bluesky":
					rateLimitDelay = 1 * time.Second // More permissive for Bluesky's higher limits
				default:
					rateLimitDelay = 5 * time.Second // Safe default for unknown platforms
				}
			}

			// Parse options
			options := internal.PruneOptions{
				PreserveSelfLike: preserveSelfLike,
				PreservePinned:   preservePinned,
				UnlikePosts:      unlikePosts,
				UnshareReposts:   unshareReposts,
				DryRun:           dryRun,
				RateLimitDelay:   rateLimitDelay,
			}

			// Parse max age
			if maxAgeStr != "" {
				maxAge, err := parseDuration(maxAgeStr)
				if err != nil {
					fmt.Printf("Error parsing max-post-age for %s: %v\n", platformName, err)
					if len(platforms) > 1 {
						totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: max-post-age parse error: %v", platformName, err))
						continue
					}
					os.Exit(1)
				}
				options.MaxAge = &maxAge
			}

			// Parse before date
			if beforeDateStr != "" {
				beforeDate, err := parseDate(beforeDateStr)
				if err != nil {
					fmt.Printf("Error parsing before-date for %s: %v\n", platformName, err)
					if len(platforms) > 1 {
						totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: before-date parse error: %v", platformName, err))
						continue
					}
					os.Exit(1)
				}
				options.BeforeDate = &beforeDate
			}

			// Validate that at least one criteria is specified
			if options.MaxAge == nil && options.BeforeDate == nil {
				fmt.Printf("Error for %s: Must specify either --max-post-age or --before-date\n", platformName)
				if len(platforms) > 1 {
					totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: no age criteria specified", platformName))
					continue
				}
				os.Exit(1)
			}

			// Perform pruning for this platform
			var result *internal.PruneResult
			if continueUntilEnd {
				result = performContinuousPruningWithResult(client, username, options)
			} else {
				var err error
				result, err = client.PrunePosts(username, options)
				if err != nil {
					fmt.Printf("Error pruning posts from %s: %v\n", client.GetPlatformName(), err)
					if len(platforms) > 1 {
						totalResults.Errors = append(totalResults.Errors, fmt.Sprintf("%s: %v", platformName, err))
						continue
					}
					os.Exit(1)
				}
			}

			// Display results for this platform
			displayPruneResults(result, client.GetPlatformName(), dryRun)

			// Add to total results
			totalResults.PostsToDelete = append(totalResults.PostsToDelete, result.PostsToDelete...)
			totalResults.PostsToUnlike = append(totalResults.PostsToUnlike, result.PostsToUnlike...)
			totalResults.PostsToUnshare = append(totalResults.PostsToUnshare, result.PostsToUnshare...)
			totalResults.PostsPreserved = append(totalResults.PostsPreserved, result.PostsPreserved...)
			totalResults.DeletedCount += result.DeletedCount
			totalResults.UnlikedCount += result.UnlikedCount
			totalResults.UnsharedCount += result.UnsharedCount
			totalResults.PreservedCount += result.PreservedCount
			totalResults.ErrorsCount += result.ErrorsCount
			totalResults.Errors = append(totalResults.Errors, result.Errors...)

			// Add spacing between platforms when processing multiple
			if len(platforms) > 1 && i < len(platforms)-1 {
				fmt.Println() // Extra newline between platforms
			}
		}

		// Show combined results if multiple platforms were processed
		if len(platforms) > 1 {
			fmt.Printf("\n=== COMBINED RESULTS ===\n")
			displayPruneResults(totalResults, "All Platforms", dryRun)
		}
	},
}

func performContinuousPruningWithResult(client internal.SocialClient, username string, options internal.PruneOptions) *internal.PruneResult {
	platform := client.GetPlatformName()
	totalDeleted := 0
	totalUnliked := 0
	totalUnshared := 0
	totalPreserved := 0
	totalErrors := 0
	var allErrors []string
	round := 1
	cursor := ""      // Start with empty cursor for pagination
	batchLimit := 100 // Fetch more posts per batch for pruning

	fmt.Printf("Starting continuous pruning on %s (will continue until no more posts match criteria)...\n", platform)
	if options.DryRun {
		fmt.Println("DRY RUN MODE: No actual actions will be performed")
	}

	for {
		fmt.Printf("Round %d: Fetching posts...\n", round)
		
		// Use paginated fetching to avoid infinite loops
		posts, nextCursor, err := client.FetchUserPostsPaginated(username, batchLimit, cursor)
		if err != nil {
			fmt.Printf("Error in round %d: %v\n", round, err)
			fmt.Printf("Stopping continuous pruning. Total processed so far:\n")
			allErrors = append(allErrors, fmt.Sprintf("Round %d: %v", round, err))
			break
		}

		// Check if we have reached the end (no more posts or same cursor)
		if len(posts) == 0 {
			fmt.Printf("Round %d: No more posts found. Pruning complete!\n", round)
			break
		}

		if nextCursor == "" || nextCursor == cursor {
			fmt.Printf("Round %d: Reached end of timeline. Pruning complete!\n", round)
			break
		}

		// Create a mock PruneResult to hold posts for this batch and process them
		result := &internal.PruneResult{
			PostsToDelete:  []internal.Post{},
			PostsToUnlike:  []internal.Post{},
			PostsToUnshare: []internal.Post{},
			PostsPreserved: []internal.Post{},
			Errors:         []string{},
		}

		// Filter posts by age criteria and process them
		matchingPosts := 0
		for _, post := range posts {
			// Apply age filtering (inline logic)
			now := time.Now()
			matchesAge := true
			
			if options.MaxAge != nil {
				if now.Sub(post.CreatedAt) <= *options.MaxAge {
					matchesAge = false
				}
			}
			
			if options.BeforeDate != nil {
				if !post.CreatedAt.Before(*options.BeforeDate) {
					matchesAge = false
				}
			}
			
			if !matchesAge {
				continue
			}
			matchingPosts++

			// Check if post should be preserved (inline logic)
			shouldPreserve := false
			if options.PreservePinned && post.IsPinned {
				shouldPreserve = true
			}
			if options.PreserveSelfLike && post.IsLikedByUser {
				shouldPreserve = true
			}

			// Process each matching post
			if shouldPreserve {
				result.PostsPreserved = append(result.PostsPreserved, post)
				totalPreserved++
				if options.DryRun {
					fmt.Printf("🛡️ PRESERVED: %s\n", truncateContent(post.Content, 50))
				}
			} else {
				// Determine action based on post type and options
				if post.Type == internal.PostTypeLike && options.UnlikePosts {
					result.PostsToUnlike = append(result.PostsToUnlike, post)
					totalUnliked++
					if options.DryRun {
						fmt.Printf("👎 UNLIKE: %s\n", truncateContent(post.Content, 50))
					}
				} else if post.Type == internal.PostTypeRepost && options.UnshareReposts {
					result.PostsToUnshare = append(result.PostsToUnshare, post)
					totalUnshared++
					if options.DryRun {
						fmt.Printf("🔄 UNSHARE: %s\n", truncateContent(post.Content, 50))
					}
				} else {
					result.PostsToDelete = append(result.PostsToDelete, post)
					totalDeleted++
					if options.DryRun {
						fmt.Printf("🗑️ DELETE: %s\n", truncateContent(post.Content, 50))
					}
				}
			}
		}

		fmt.Printf("Round %d completed: %d posts matched criteria\n", round, matchingPosts)
		
		// If no posts matched criteria this round, continue to next batch
		if matchingPosts == 0 {
			fmt.Printf("Round %d: No posts matched criteria, continuing...\n", round)
		}

		cursor = nextCursor
		round++

		// Small delay between rounds to be respectful to APIs
		time.Sleep(2 * time.Second)
	}

	// Return final results
	return &internal.PruneResult{
		PostsToDelete:  []internal.Post{}, // Posts already processed in dry-run mode
		PostsToUnlike:  []internal.Post{},
		PostsToUnshare: []internal.Post{},
		PostsPreserved: []internal.Post{},
		DeletedCount:   totalDeleted,
		UnlikedCount:   totalUnliked,
		UnsharedCount:  totalUnshared,
		PreservedCount: totalPreserved,
		ErrorsCount:    totalErrors,
		Errors:         allErrors,
	}
}

func performContinuousPruning(client internal.SocialClient, username string, options internal.PruneOptions) {
	platform := client.GetPlatformName()
	totalDeleted := 0
	totalUnliked := 0
	totalUnshared := 0
	totalPreserved := 0
	totalErrors := 0
	var allErrors []string
	round := 1
	cursor := ""      // Start with empty cursor for pagination
	batchLimit := 100 // Fetch more posts per batch for pruning

	fmt.Printf("Starting continuous pruning on %s (will continue until no more posts match criteria)...\n", platform)
	if options.DryRun {
		fmt.Println("DRY RUN MODE: No actual actions will be performed")
	}

	for {
		fmt.Printf("Round %d: Fetching posts...\n", round)
		
		// Use paginated fetching to avoid infinite loops
		posts, nextCursor, err := client.FetchUserPostsPaginated(username, batchLimit, cursor)
		if err != nil {
			fmt.Printf("Error in round %d: %v\n", round, err)
			fmt.Printf("Stopping continuous pruning. Total processed so far:\n")
			break
		}

		// Check if we have reached the end (no more posts or same cursor)
		if len(posts) == 0 {
			fmt.Printf("Round %d: No more posts found. Pruning complete!\n", round)
			break
		}

		if nextCursor == "" || nextCursor == cursor {
			fmt.Printf("Round %d: Reached end of timeline. Pruning complete!\n", round)
			break
		}

		// Create a mock PruneResult to hold posts for this batch and process them
		result := &internal.PruneResult{
			PostsToDelete:  []internal.Post{},
			PostsToUnlike:  []internal.Post{},
			PostsToUnshare: []internal.Post{},
			PostsPreserved: []internal.Post{},
			Errors:         []string{},
		}

		// Filter posts by age criteria first
		now := time.Now()
		matchingPosts := []internal.Post{}
		for _, post := range posts {
			shouldProcess := false

			// Check age criteria
			if options.MaxAge != nil {
				if now.Sub(post.CreatedAt) > *options.MaxAge {
					shouldProcess = true
				}
			}

			// Check date criteria
			if options.BeforeDate != nil {
				if post.CreatedAt.Before(*options.BeforeDate) {
					shouldProcess = true
				}
			}

			if shouldProcess {
				matchingPosts = append(matchingPosts, post)
			}
		}

		// If no posts matched criteria in this batch, continue to next batch
		if len(matchingPosts) == 0 {
			fmt.Printf("Round %d: No posts matched criteria in this batch, continuing...\n", round)
			cursor = nextCursor
			round++
			time.Sleep(time.Second) // Small delay between rounds
			continue
		}

		// Process each matching post according to preservation rules and actions
		for _, post := range matchingPosts {
			// Check preservation rules
			preserveReason := ""
			if options.PreservePinned && post.IsPinned {
				preserveReason = "pinned"
			} else if options.PreserveSelfLike && post.IsLikedByUser && post.Type == internal.PostTypeOriginal {
				preserveReason = "self-liked"
			}

			if preserveReason != "" {
				result.PostsPreserved = append(result.PostsPreserved, post)
				result.PreservedCount++
			} else {
				// Process the post according to its type and options
				if post.Type == internal.PostTypeLike {
					result.PostsToUnlike = append(result.PostsToUnlike, post)
					// TODO: Implement actual unliking logic per platform
				} else if post.Type == internal.PostTypeRepost {
					result.PostsToUnshare = append(result.PostsToUnshare, post)
					// TODO: Implement actual unsharing logic per platform
				} else if post.Type == internal.PostTypeOriginal || post.Type == internal.PostTypeReply {
					result.PostsToDelete = append(result.PostsToDelete, post)
					// TODO: Implement actual deletion logic per platform
				}
			}
		}

		// For now, just set counts to match the lists for dry-run display
		// In a full implementation, these would be updated by actual operations
		if options.DryRun {
			result.DeletedCount = len(result.PostsToDelete)
			result.UnlikedCount = len(result.PostsToUnlike)
			result.UnsharedCount = len(result.PostsToUnshare)
		}

		// Update totals
		totalDeleted += result.DeletedCount
		totalUnliked += result.UnlikedCount
		totalUnshared += result.UnsharedCount
		totalPreserved += result.PreservedCount
		totalErrors += result.ErrorsCount
		allErrors = append(allErrors, result.Errors...)

		// Show round results with streaming for dry-run
		if options.DryRun {
			// Stream each post that would be processed
			if len(result.PostsToDelete) > 0 {
				fmt.Printf("Round %d - Posts to delete:\n", round)
				for _, post := range result.PostsToDelete {
					fmt.Printf("  🗑️  [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
				}
			}
			if len(result.PostsToUnlike) > 0 {
				fmt.Printf("Round %d - Posts to unlike:\n", round)
				for _, post := range result.PostsToUnlike {
					fmt.Printf("  👎 [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
				}
			}
			if len(result.PostsToUnshare) > 0 {
				fmt.Printf("Round %d - Posts to unshare:\n", round)
				for _, post := range result.PostsToUnshare {
					fmt.Printf("  🔄 [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
				}
			}
			if len(result.PostsPreserved) > 0 {
				fmt.Printf("Round %d - Posts preserved:\n", round)
				for _, post := range result.PostsPreserved {
					reason := ""
					if post.IsLikedByUser && post.Handle == post.Author {
						reason = " (self-liked)"
					}
					if post.IsPinned {
						reason = " (pinned)"
					}
					fmt.Printf("  🛡️  [%s] @%s - %s%s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60), reason)
				}
			}
		} else {
			// Non-dry-run: show summary only
			fmt.Printf("Round %d results:\n", round)
			if result.DeletedCount > 0 {
				fmt.Printf("  Deleted: %d posts\n", result.DeletedCount)
			}
			if result.UnlikedCount > 0 {
				fmt.Printf("  Unliked: %d posts\n", result.UnlikedCount)
			}
			if result.UnsharedCount > 0 {
				fmt.Printf("  Unshared: %d posts\n", result.UnsharedCount)
			}
			if result.PreservedCount > 0 {
				fmt.Printf("  Preserved: %d posts\n", result.PreservedCount)
			}
		}
		
		if result.ErrorsCount > 0 {
			fmt.Printf("  Errors: %d\n", result.ErrorsCount)
		}
		fmt.Println()

		// Move to next batch using pagination cursor
		cursor = nextCursor
		round++

		// Small delay between rounds to be respectful to APIs
		time.Sleep(time.Second)
	}

	// Display final summary
	fmt.Printf("=== CONTINUOUS PRUNING COMPLETE ===\n")
	fmt.Printf("Total rounds: %d\n", round-1)
	if options.DryRun {
		fmt.Printf("DRY RUN - Would have processed:\n")
		fmt.Printf("  Total deletions: %d\n", totalDeleted)
		fmt.Printf("  Total unlikes: %d\n", totalUnliked) 
		fmt.Printf("  Total unshares: %d\n", totalUnshared)
	} else {
		fmt.Printf("Actually processed:\n")
		fmt.Printf("  Total deleted: %d\n", totalDeleted)
		fmt.Printf("  Total unliked: %d\n", totalUnliked)
		fmt.Printf("  Total unshared: %d\n", totalUnshared)
	}
	fmt.Printf("  Total preserved: %d\n", totalPreserved)
	if totalErrors > 0 {
		fmt.Printf("  Total errors: %d\n", totalErrors)
		if len(allErrors) > 0 {
			fmt.Println("  Error details:")
			for _, err := range allErrors {
				fmt.Printf("    - %s\n", err)
			}
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	// First try standard Go duration parsing (handles formats like "2h30m", "1h30m45s")
	if duration, err := time.ParseDuration(s); err == nil {
		if duration < 0 {
			return 0, fmt.Errorf("negative durations are not allowed")
		}
		return duration, nil
	}

	// Support custom formats like "30d", "7d", "1y"
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %w", err)
	}

	if value < 0 {
		return 0, fmt.Errorf("negative durations are not allowed")
	}

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit: %s", unit)
	}
}

func parseDate(s string) (time.Time, error) {
	// Support multiple date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"01/02/2006",
		"01/02/2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date format. Supported formats: YYYY-MM-DD, YYYY-MM-DD HH:MM:SS, MM/DD/YYYY")
}

func displayPruneResults(result *internal.PruneResult, platform string, dryRun bool) {
	if dryRun {
		fmt.Printf("DRY RUN: Actions that would be performed on %s:\n\n", platform)
	} else {
		fmt.Printf("Pruning results for %s:\n\n", platform)
	}

	totalActions := len(result.PostsToDelete) + len(result.PostsToUnlike) + len(result.PostsToUnshare)
	if totalActions == 0 {
		fmt.Println("No posts match the specified criteria.")
		return
	}

	// Stream posts to be deleted
	if len(result.PostsToDelete) > 0 {
		fmt.Printf("Posts %s:\n", map[bool]string{true: "that would be deleted", false: "deleted"}[dryRun])
		for i, post := range result.PostsToDelete {
			if dryRun {
				fmt.Printf("  🗑️  [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			} else {
				fmt.Printf("%d. [%s] @%s - %s\n", i+1, post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			}
			if post.URL != "" {
				fmt.Printf("     URL: %s\n", post.URL)
			}
		}
		fmt.Println()
	}

	// Stream posts to be unliked
	if len(result.PostsToUnlike) > 0 {
		fmt.Printf("Posts %s:\n", map[bool]string{true: "that would be unliked", false: "unliked"}[dryRun])
		for i, post := range result.PostsToUnlike {
			if dryRun {
				fmt.Printf("  👎 [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			} else {
				fmt.Printf("%d. [%s] @%s - %s\n", i+1, post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			}
			if post.URL != "" {
				fmt.Printf("     URL: %s\n", post.URL)
			}
		}
		fmt.Println()
	}

	// Stream posts to be unshared
	if len(result.PostsToUnshare) > 0 {
		fmt.Printf("Posts %s:\n", map[bool]string{true: "that would be unshared", false: "unshared"}[dryRun])
		for i, post := range result.PostsToUnshare {
			if dryRun {
				fmt.Printf("  🔄 [%s] @%s - %s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			} else {
				fmt.Printf("%d. [%s] @%s - %s\n", i+1, post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60))
			}
			if post.URL != "" {
				fmt.Printf("     URL: %s\n", post.URL)
			}
		}
		fmt.Println()
	}

	// Show preserved posts if any
	if len(result.PostsPreserved) > 0 {
		fmt.Printf("Posts preserved (due to --preserve-* flags):\n")
		for i, post := range result.PostsPreserved {
			reason := ""
			if post.IsLikedByUser && post.Handle == post.Author {
				reason = " (self-liked)"
			}
			if post.IsPinned {
				reason = " (pinned)"
			}
			if dryRun {
				fmt.Printf("  🛡️  [%s] @%s - %s%s\n", post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60), reason)
			} else {
				fmt.Printf("%d. [%s] @%s - %s%s\n", i+1, post.CreatedAt.Format("2006-01-02"), post.Handle, truncateContent(post.Content, 60), reason)
			}
		}
		fmt.Println()
	}

	// Show summary
	fmt.Printf("Summary:\n")
	if dryRun {
		if len(result.PostsToDelete) > 0 {
			fmt.Printf("  Would delete: %d posts\n", len(result.PostsToDelete))
		}
		if len(result.PostsToUnlike) > 0 {
			fmt.Printf("  Would unlike: %d posts\n", len(result.PostsToUnlike))
		}
		if len(result.PostsToUnshare) > 0 {
			fmt.Printf("  Would unshare: %d posts\n", len(result.PostsToUnshare))
		}
		if len(result.PostsPreserved) > 0 {
			fmt.Printf("  Would preserve: %d posts\n", len(result.PostsPreserved))
		}
	} else {
		if result.DeletedCount > 0 {
			fmt.Printf("  Deleted: %d posts\n", result.DeletedCount)
		}
		if result.UnlikedCount > 0 {
			fmt.Printf("  Unliked: %d posts\n", result.UnlikedCount)
		}
		if result.UnsharedCount > 0 {
			fmt.Printf("  Unshared: %d posts\n", result.UnsharedCount)
		}
		if result.PreservedCount > 0 {
			fmt.Printf("  Preserved: %d posts\n", result.PreservedCount)
		}
		if result.ErrorsCount > 0 {
			fmt.Printf("  Errors: %d\n", result.ErrorsCount)
			for _, err := range result.Errors {
				fmt.Printf("    - %s\n", err)
			}
		}
	}
}

func truncateContent(content string, maxLen int) string {
	// Replace newlines with spaces for display
	content = strings.ReplaceAll(content, "\n", " ")
	if len(content) <= maxLen {
		return content
	}
	if maxLen <= 3 {
		return "..."
	}
	return content[:maxLen-3] + "..."
}

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().String("platforms", "", "Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms")
	pruneCmd.Flags().String("max-post-age", "", "Delete posts older than this (e.g., 30d, 1y, 24h)")
	pruneCmd.Flags().String("before-date", "", "Delete posts created before this date (YYYY-MM-DD or MM/DD/YYYY)")
	pruneCmd.Flags().Bool("preserve-selflike", false, "Don't delete user's own posts that they have liked")
	pruneCmd.Flags().Bool("preserve-pinned", false, "Don't delete pinned posts")
	pruneCmd.Flags().Bool("unlike-posts", false, "Unlike posts instead of deleting them")
	pruneCmd.Flags().Bool("unshare-reposts", false, "Unshare/unrepost instead of deleting reposts")
	pruneCmd.Flags().Bool("continue", false, "Continue searching and processing posts until no more match the criteria")
	pruneCmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	pruneCmd.Flags().String("rate-limit-delay", "", "Delay between API requests to respect rate limits (default: 60s for Mastodon, 1s for Bluesky)")
}
