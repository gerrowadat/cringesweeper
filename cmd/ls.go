package cmd

import (
	"fmt"
	"os"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [username]",
	Short: "List recent posts from social media timelines",
	Long: `List and display recent posts from a user's social media timeline.

Supports multiple platforms including Bluesky and Mastodon. Shows post content,
timestamps, author information, and post types (original, repost, reply, etc.).

The username can be provided as an argument or via environment variables.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		username := ""
		
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

		posts, err := client.FetchUserPosts(username, 10)
		if err != nil {
			fmt.Printf("Error fetching posts from %s: %v\n", client.GetPlatformName(), err)
			os.Exit(1)
		}

		displayPosts(posts, client.GetPlatformName())
	},
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
}