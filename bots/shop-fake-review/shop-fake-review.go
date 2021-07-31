package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

var (
	pronouns = []string{"I", "My family", "My wife", "My dog"}
	verbs = []string{"tried out", "bought", "used", "utilized"}
	duration = []string{"1 month", "2 days", "1 year"}
	names = []string{"Petya", "Sasha", "Grisha", "Misha", "Oleg"}
)

var (
	ErrTooQuickly  = errors.New("Posting too quickly")
	ErrDuplicate   = errors.New("Duplicate comment")
	ErrMaintenance = errors.New("Site on maintenance")
)

func main() {
	productLink := flag.String("product", "", "Link to the product for which to leave fake reviews for.")
	reviewAmount := flag.Int("n", 10, "Amount of reviews to leave.")
	rating := flag.Int("rating", 5, "Rating for the reviews (1-5).")

	flag.Parse()

	if *productLink == "" || *reviewAmount <= 0 {
		flag.Usage()
		return
	}

	if *rating < 1 {
		*rating = 1
	} else if *rating > 5 {
		*rating = 5
	}

	prodURL, err := url.Parse(*productLink)
	if err != nil {
		log.Fatal(err)
	}

	postID, err := getPostID(*productLink)
	if err != nil {
		log.Fatal(err)
	}

	timeoutDefault := 10 * time.Second
	timeoutCur := timeoutDefault
	timeoutLimit := 1_000 * time.Second

	reviewsDone := 0
	for { // for reviewsDone < *reviewAmount
		log.Println("Sending review")
		err := leaveReview(prodURL.Host, postID, *rating)
		switch err {
		case ErrDuplicate:
			log.Println("Tried to send duplicate review")

		case ErrMaintenance:
			log.Println("Site under maintenance")
			if timeoutCur < timeoutLimit {
				timeoutCur *= 2
			}
			log.Printf("Sleeping for %v\n", timeoutCur)
			time.Sleep(timeoutCur)

		case ErrTooQuickly:
			log.Println("Got timed out")
			if timeoutCur < timeoutLimit {
				timeoutCur *= 2
			}
			log.Printf("Sleeping for %v\n", timeoutCur)
			time.Sleep(timeoutCur)

		case nil:
			timeoutCur = timeoutDefault
			reviewsDone++
			log.Printf("Sent review")
			if reviewsDone >= *reviewAmount {
				return
			}
			log.Printf("Sleeping for %v\n", timeoutCur)
			time.Sleep(timeoutCur)

		default:
			log.Fatal(err)
		}
	}
}

func getPostID(productLink string) (string, error) {
	resp, err := http.DefaultClient.Get(productLink)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Extract value of attribute "value" from tag <input> with attribute name="comment_post_ID"
	// <input type="hidden" name="comment_post_ID" value="15" id="comment_post_ID">
	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		tn, hasAttr := z.TagName()
		if string(tn) != "input" || !hasAttr {
			continue
		}

		var hasPostID bool
		var postID string

		var key, val []byte
		moreAttr := true
		for moreAttr {
			key, val, moreAttr = z.TagAttr()

			if string(key) == "name" && string(val) == "comment_post_ID" {
				hasPostID = true
			}

			if string(key) == "value" {
				postID = string(val)
			}

			if hasPostID && len(postID) > 0 {
				return postID, nil
			}
		}
	}

	return "", errors.New("Post ID not found")
}

func leaveReview(host, postID string, rating int) error {
	to := fmt.Sprintf("http://%s/wp-comments-post.php", host)

	author := genAuthor()

	values := url.Values{}
	values.Set("rating", strconv.Itoa(rating))
	values.Set("comment", genReview())
	values.Set("author", author)
	values.Set("email", genEmail(author))
	values.Set("submit", "Submit")
	values.Set("comment_post_ID", postID)
	values.Set("comment_parent", "0")

	// NOTE: program can be parallelized if proxies are used
	client := &http.Client{}
	resp, err := client.PostForm(to, values)
	if resp == nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	switch {
	case bytes.Contains(body, []byte("Duplicate comment")):
		return ErrDuplicate
	case bytes.Contains(body, []byte("Slow down")):
		return ErrTooQuickly
	case bytes.Contains(body, []byte("scheduled maintenance")):
		return ErrMaintenance
	}

	return nil
}

func genAuthor() string {
	name := names[rand.Intn(len(names))]
	return fmt.Sprintf("%s%d", name, rand.Intn(1_000_000))
}

func genEmail(author string) string {
	return fmt.Sprintf("%s@example.com", author)
}

func genReview() string {
	f := pronouns[rand.Intn(len(pronouns))]
	s := verbs[rand.Intn(len(verbs))]
	t := duration[rand.Intn(len(duration))]

	review := fmt.Sprintf("%s %s this product for around %s", f, s, t)
	return review
}
