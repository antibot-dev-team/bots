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
	"regexp"

	"github.com/pkg/errors"
)

var (
	pronouns = []string{"I", "My family", "My wife", "My dog"}
	verbs = []string{"tried out", "bought", "used", "utilized"}
	durations = []string{"1 month", "2 days", "1 year"}
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
		err := leaveReview(prodURL.Scheme, prodURL.Host, postID, *rating)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Extract product ID as a content of field 'p' from:
	// <link rel="shortlink" href="http://localhost/?p=32">
	re := regexp.MustCompile("[?&]p=(\\d+)[\"&']")
	matches := re.FindSubmatch(body)
	if len(matches) != 2 {
		return "", errors.New("Could not find post ID")
	}
	prodID := string(matches[1])

	return prodID, nil
}

func leaveReview(scheme, host, postID string, rating int) error {
	to := fmt.Sprintf("%s://%s/wp-comments-post.php", scheme, host)

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
	case bytes.Contains(bytes.ToLower(body), []byte("duplicate comment")):
		return ErrDuplicate
	case bytes.Contains(bytes.ToLower(body), []byte("slow down")):
		return ErrTooQuickly
	case bytes.Contains(bytes.ToLower(body), []byte("scheduled maintenance")):
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
	pronoun := pronouns[rand.Intn(len(pronouns))]
	verb := verbs[rand.Intn(len(verbs))]
	duration := durations[rand.Intn(len(durations))]

	review := fmt.Sprintf("%s %s this product for around %s", pronoun, verb, duration)
	return review
}
