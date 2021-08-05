package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	pronouns  = []string{"I", "My family", "My wife", "My dog"}
	verbs     = []string{"tried out", "bought", "used", "utilized"}
	durations = []string{"1 month", "2 days", "1 year"}
	names     = []string{"Petya", "Sasha", "Grisha", "Misha", "Oleg"}
)

var (
	ErrTooQuickly  = errors.New("Posting too quickly")
	ErrDuplicate   = errors.New("Duplicate comment")
	ErrMaintenance = errors.New("Site on maintenance")
)

func main() {
	productLink := flag.String("product", "", "Link to the product for which to leave fake reviews for (including scheme).")
	reviewAmount := flag.Uint("n", 1, "Amount of reviews to leave from single proxy.")
	maxAttempts := flag.Uint("attempts", 3, "Amount of attempts allowed to send single review. Zero means no limit.")
	rating := flag.Int("rating", 5, "Rating for the reviews (1-5).")
	proxyPath := flag.String("proxy", "", "Path to text file containing one proxy per line. "+
		"Supported schemes: http, https, socks5. Line example: socks5://0.0.0.0:1337."+
		"If no path specified or no proxies in file present, proxy from $HTTP_PROXY is used.")
	reqTimeout := flag.Duration("timeout", 10*time.Second, "The time given to the bot for one request.")
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

	if prodURL.Scheme == "" {
		log.Fatal("Given product url doesn't have a scheme.")
	}

	proxies, err := scanProxies(*proxyPath)
	if err != nil {
		log.Fatal(err)
	}

	// Get postID using first proxy in the given list, or without proxy if no proxies are given
	var postID string
	var client *http.Client
	if proxies == nil || len(proxies) == 0 {
		client = &http.Client{Timeout: *reqTimeout}
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxies[0]),
			},
			Timeout: *reqTimeout,
		}
	}

	postID, err = getPostID(client, *productLink)
	if err != nil {
		log.Fatal(err)
	}

	wg := &sync.WaitGroup{}

	// Case where no proxies are given
	// WaitGroup here is redundant, but present to reuse startBot function with no proxies
	if proxies == nil || len(proxies) == 0 {
		wg.Add(1)
		startBot(wg, client, prodURL, postID, *rating, *maxAttempts, *reviewAmount, 0)
		return
	}

	for botID, proxy := range proxies {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxy),
			},
			Timeout: *reqTimeout,
		}
		wg.Add(1)
		go startBot(wg, client, prodURL, postID, *rating, *maxAttempts, *reviewAmount, botID)
	}

	wg.Wait()
}

func scanProxies(proxyPath string) ([]*url.URL, error) {
	var proxies []*url.URL

	if proxyPath == "" {
		return nil, nil
	}

	proxyFile, err := os.Open(proxyPath)
	if err != nil {
		return nil, err
	}
	defer proxyFile.Close()

	scanner := bufio.NewScanner(proxyFile)
	for scanner.Scan() {
		line := scanner.Text()
		proxy, err := url.Parse(line)
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, proxy)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return proxies, nil
}

func startBot(wg *sync.WaitGroup, client *http.Client, prodURL *url.URL, postID string, rating int, maxAttempts, reviewAmount uint, botID int) {
	timeoutDefault := 15 * time.Second
	var reviewsDone uint
	var attempts uint
	for { // for reviewsDone < *reviewAmount
		log.Printf("[BOT-%d] Sending reviev\n", botID)
		err := postReview(client, prodURL.Scheme, prodURL.Host, postID, rating)
		switch err {
		case ErrDuplicate:
			log.Printf("[BOT-%d] Tried to send duplicate review\n", botID)

		case ErrMaintenance:
			log.Printf("[BOT-%d] Site under maintenance\n", botID)
			attempts++
			if maxAttempts != 0 && attempts >= maxAttempts {
				log.Printf("[BOT-%d] Attempts exceeded\n", botID)
				wg.Done()
				return
			}
			log.Printf("[BOT-%d] Sleeping for %v\n", botID, timeoutDefault)
			time.Sleep(timeoutDefault)

		case ErrTooQuickly:
			log.Printf("[BOT-%d] Got timed out\n", botID)
			attempts++
			if maxAttempts != 0 && attempts >= maxAttempts {
				log.Printf("[BOT-%d] Attempts exceeded\n", botID)
				wg.Done()
				return
			}
			timeout := timeoutDefault + time.Duration(rand.Intn(5))*time.Second
			log.Printf("[BOT-%d] Sleeping for %v\n", botID, timeout)
			time.Sleep(timeout)

		case nil:
			reviewsDone++
			attempts = 0
			log.Printf("[BOT-%d] Sent review\n", botID)
			if reviewsDone >= reviewAmount {
				wg.Done()
				return
			}
			log.Printf("[BOT-%d] Sleeping for %v\n", botID, timeoutDefault)
			time.Sleep(timeoutDefault)

		default:
			log.Printf("[BOT-%d] Got error: %v", botID, err)
			wg.Done()
			return
		}
	}
}

func getPostID(client *http.Client, productLink string) (string, error) {
	resp, err := client.Get(productLink)
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

func postReview(client *http.Client, scheme, host, postID string, rating int) error {
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
