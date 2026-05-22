package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	r2Only := len(os.Args) > 1 && (os.Args[1] == "r2" || os.Args[1] == "storage" || os.Args[1] == "cloudflare")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if !r2Only {
		if err := pingMongo(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "MONGODB_URI: FAIL — %v\n", err)
			os.Exit(1)
		}
		fmt.Println("MONGODB_URI: OK (ping)")
	}

	warn, err := smokeCloudflareR2(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cloudflare R2: FAIL — %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Cloudflare R2: OK (S3 API + cleanup)")
	for _, w := range warn {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", w)
	}
}

func pingMongo(ctx context.Context) error {
	uri := strings.TrimSpace(os.Getenv("MONGODB_URI"))
	if uri == "" {
		return fmt.Errorf("not set")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	return client.Ping(ctx, nil)
}

func smokeCloudflareR2(ctx context.Context) (warnings []string, err error) {
	typ := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_TYPE")))
	if typ != "s3" && typ != "r2" {
		return nil, fmt.Errorf("STORAGE_TYPE=%q — use s3 (R2 is S3-compatible)", typ)
	}
	ep := strings.TrimSpace(os.Getenv("STORAGE_ENDPOINT"))
	access := strings.TrimSpace(os.Getenv("STORAGE_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("STORAGE_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("STORAGE_BUCKET"))
	if ep == "" || access == "" || secret == "" || bucket == "" {
		return nil, fmt.Errorf("missing STORAGE_ENDPOINT, ACCESS_KEY, SECRET_KEY, or BUCKET")
	}
	if !strings.Contains(ep, "r2.cloudflarestorage.com") && !strings.Contains(strings.ToLower(ep), "r2") {
		fmt.Fprintf(os.Stderr, "Note: STORAGE_ENDPOINT does not look like R2 (%q); still testing S3 API.\n", ep)
	}
	if !strings.HasPrefix(ep, "http") {
		ep = "https://" + ep
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(access, secret, ""),
		BaseEndpoint: aws.String(ep),
		UsePathStyle: true,
	})
	key := fmt.Sprintf(".smoke-env/%d.txt", time.Now().UnixNano())
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("rloko cloudflare r2 smoke"),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return nil, fmt.Errorf("PutObject: %w", err)
	}
	fmt.Println("  S3 API (R2): PutObject OK")

	publicBase := strings.TrimSuffix(strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_URL")), "/")
	if publicBase != "" {
		url := publicBase + "/" + key
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		if err != nil {
			return nil, fmt.Errorf("public URL request: %w", err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Public URL HEAD failed: %v — check r2.dev / custom domain allows anonymous reads", err))
		} else {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				warnings = append(warnings, fmt.Sprintf("Public URL %s → %s — in Cloudflare: R2 bucket → Settings → Public access, or attach the correct r2.dev subdomain", url, res.Status))
			} else {
				fmt.Println("  Public URL (r2.dev): HEAD OK")
			}
		}
	} else {
		fmt.Println("  STORAGE_PUBLIC_URL unset — skipped public URL check")
	}

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return warnings, fmt.Errorf("DeleteObject: %w", err)
	}
	fmt.Println("  S3 API (R2): DeleteObject OK")
	return warnings, nil
}
