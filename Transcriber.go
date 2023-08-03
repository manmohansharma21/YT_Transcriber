package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/transcribeservice"
	"github.com/kkdai/youtube/v2"
)

func main() {
	transcriber()
}
func transcriber() {
	// Check if a YouTube video URL is provided as an argument
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <YouTube_video_URL>")
		return
	}

	videoURL := os.Args[1]
	videoID, _ := extractVideoIDFromURL(videoURL)
	// Initialize the AWS session using credentials
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Change this to your desired AWS region
		// Provide your AWS access key ID and secret access key here or set them as environment variables
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		),
	})
	if err != nil {
		fmt.Println("Error creating AWS session:", err)
		return
	}

	// Create an AWS Transcribe service client
	transcribeClient := transcribeservice.New(sess)

	// Create an S3 bucket for storing the YouTube video audio
	videoID = strings.ReplaceAll(videoID, "-", "_")
	bucketName := generateCleanBucketName(videoID)
	err = createS3Bucket(sess, bucketName)
	if err != nil {
		fmt.Println("Error creating S3 bucket:", err)
		fmt.Println("bucketName:", bucketName)
		return
	}
	fmt.Println("S3 Bucket created:", bucketName)

	// Download the YouTube video audio and upload it to the S3 bucket
	audioFileURI, err := downloadYouTubeAudio(sess, bucketName, videoURL)
	if err != nil {
		fmt.Println("Error downloading YouTube audio:", err)
		return
	}
	fmt.Println("YouTube audio downloaded and uploaded to S3:", audioFileURI)

	transcriptionJobName := "YouTubeTranscription-" + videoID + "-" + time.Now().Format("20060102150405")

	// Delete the existing transcription job with the same videoID
	deleteExistingTranscriptionJob(transcribeClient, transcriptionJobName)

	// Start a transcription job
	transcriptionJobResponse, err := transcribeClient.StartTranscriptionJob(&transcribeservice.StartTranscriptionJobInput{
		TranscriptionJobName: &transcriptionJobName,
		LanguageCode:         aws.String("en-US"), // Change this to match the language of the video if needed
		Media: &transcribeservice.Media{
			MediaFileUri: aws.String(audioFileURI),
		},
	})
	if err != nil {
		fmt.Println("Error starting transcription job:", err)
		return
	}

	fmt.Println("Transcription Job Name:", *transcriptionJobResponse.TranscriptionJob.TranscriptionJobName)

	// Wait for the transcription job to complete
	waitForTranscriptionJob(transcribeClient, transcriptionJobName)

	// Get the transcription job result
	transcriptS3URL := getTranscriptionJobResult(transcribeClient, transcriptionJobName)

	// Get the transcript from S3
	// transcriptJSON, err := getTranscriptFromS3(sess, transcriptionJobName, bucketName)
	// if err != nil {
	// 	fmt.Println("Error getting transcript from S3:", err)
	// 	return
	// }

	// // Process the transcript JSON as needed
	// fmt.Println("Transcript JSON:")
	// fmt.Println(string(transcriptJSON))

	// Get the transcript from AWS Transcribe
	transcriptJSON, err := getTranscriptFromS3(sess, transcriptionJobName, "aws-transcribe-us-east-1-prod")
	if err != nil {
		fmt.Println("Error getting transcript from S3:", err)
		return
	}

	// Upload the transcript to your S3 bucket
	transcriptObjectName := "youtube_video_transcript.json" // Object key for the transcript file in your S3 bucket
	err = uploadTranscriptToS3(sess, bucketName, transcriptObjectName, transcriptJSON)
	if err != nil {
		fmt.Println("Error uploading transcript to S3:", err)
		return
	}
	fmt.Println("Transcript uploaded to S3:", "s3://"+bucketName+"/"+transcriptObjectName)

	// Download the transcript from the AWS S3 URL
	transcriptFile, err := downloadTranscriptFromS3URL(transcriptS3URL)
	if err != nil {
		fmt.Println("Error downloading transcript from S3:", err)
		return
	}

	// Upload the downloaded transcript file to your S3 bucket
	downloadedObjectName := "downloaded_transcript.json" // Object key for the downloaded transcript file in your S3 bucket
	err = uploadTranscriptToS3(sess, bucketName, downloadedObjectName, transcriptFile)
	if err != nil {
		fmt.Println("Error uploading downloaded transcript to S3:", err)
		return
	}
	fmt.Println("Downloaded transcript uploaded to S3:", "s3://"+bucketName+"/"+downloadedObjectName)

}

func downloadTranscriptFromS3URL(transcriptS3URL string) ([]byte, error) {
	// Parse the transcriptS3URL to get the bucket and object key
	parsedURL, err := url.Parse(transcriptS3URL)
	if err != nil {
		return nil, err
	}
	bucketName := parsedURL.Host
	objectKey := strings.TrimPrefix(parsedURL.Path, "/")

	// Create an S3 session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Change this to your desired AWS region
		// Provide your AWS access key ID and secret access key here or set them as environment variables
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		),
	}))

	// Download the transcript file from AWS S3
	return getTranscriptFromS3(sess, objectKey, bucketName)
}

func extractVideoIDFromURL(youtubeURL string) (string, error) {
	parsedURL, err := url.Parse(youtubeURL)
	if err != nil {
		return "", err
	}

	queryParams, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		return "", err
	}

	videoID := queryParams.Get("v")
	if videoID == "" {
		return "", fmt.Errorf("video ID not found in the URL")
	}

	return videoID, nil
}

func generateCleanBucketName(videoID string) string {
	bucketNamePrefix := "youtube"
	cleanedVideoID := strings.ReplaceAll(videoID, "_", "-")
	cleanedVideoID = strings.ReplaceAll(cleanedVideoID, "-", "")
	videoID = bucketNamePrefix + cleanedVideoID
	videoID = strings.ToLower(videoID)

	maxBucketNameLength := 63
	if len(videoID) > maxBucketNameLength {
		videoID = videoID[:maxBucketNameLength]
	}

	return videoID
}

func deleteExistingTranscriptionJob(svc *transcribeservice.TranscribeService, jobName string) {
	_, err := svc.DeleteTranscriptionJob(&transcribeservice.DeleteTranscriptionJobInput{
		TranscriptionJobName: &jobName,
	})
	if err != nil {
		// Ignore errors, as the job might not exist or is already deleted.
	}
}

// CreateS3Bucket creates a new S3 bucket if it does not already exist
func createS3Bucket(sess *session.Session, bucketName string) error {
	svc := s3.New(sess)

	// Check if the bucket already exists
	_, err := svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err == nil {
		// Bucket already exists, no need to create
		return nil
	}

	// If the error is "NotFound", the bucket does not exist, so create it
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
		_, err := svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return err
		}
		return nil
	}

	// For other errors, return the error
	return err
}

// DownloadYouTubeAudio downloads the audio from a YouTube video and uploads it to the specified S3 bucket
func downloadYouTubeAudio(sess *session.Session, bucketName, videoURL string) (string, error) {
	// Create a new YouTube client
	client := youtube.Client{}
	// Get the video ID from the YouTube video URL
	//videoID := extractVideoID(videoURL)
	videoID, _ := extractVideoIDFromURL(videoURL)

	// Get the video info
	video, err := client.GetVideo(videoID)
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %v", err)
	}

	// Find the audio format with the highest quality (preference: "audio/webm; codecs=opus", then "audio/webm", and finally "audio/mp4")
	var audioFormat *youtube.Format
	for _, format := range video.Formats {
		if strings.HasPrefix(format.MimeType, "audio/webm; codecs=opus") {
			audioFormat = &format
			break
		} else if strings.HasPrefix(format.MimeType, "audio/webm") && audioFormat == nil {
			audioFormat = &format
		} else if strings.HasPrefix(format.MimeType, "audio/mp4") && audioFormat == nil {
			audioFormat = &format
		}
	}

	if audioFormat == nil {
		return "", fmt.Errorf("no suitable audio format found")
	}

	// Download the audio stream
	//stream, _, err := client.GetStream(video, &video.Formats[0])
	stream, _, err := client.GetStream(video, audioFormat)
	if err != nil {
		return "", fmt.Errorf("failed to get audio stream: %v", err)
	}
	defer stream.Close()

	audioFileURI := "s3://" + bucketName + "/" + videoID + ".mp3"
	err = uploadAudioToS3(sess, bucketName, videoID+".mp3", stream)
	if err != nil {
		return "", fmt.Errorf("failed to upload audio to S3: %v", err)
	}

	return audioFileURI, nil
}

// UploadAudioToS3 uploads the audio stream to the specified S3 bucket with the given object key
func uploadAudioToS3(sess *session.Session, bucketName, objectKey string, audioStream io.Reader) error {
	svc := s3.New(sess)

	// Create a buffer to hold the audio data
	var buf bytes.Buffer

	// Read the audio stream into the buffer
	_, err := io.Copy(&buf, audioStream)
	if err != nil {
		return fmt.Errorf("failed to read audio stream: %v", err)
	}

	// Upload the audio buffer to S3
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("failed to upload audio to S3: %v", err)
	}

	return nil
}

// Wait for the transcription job to complete
func waitForTranscriptionJob(svc *transcribeservice.TranscribeService, jobName string) {
	for {
		jobStatus, err := svc.GetTranscriptionJob(&transcribeservice.GetTranscriptionJobInput{
			TranscriptionJobName: &jobName,
		})
		if err != nil {
			fmt.Println("Error retrieving transcription job status:", err)
			os.Exit(1)
		}
		if *jobStatus.TranscriptionJob.TranscriptionJobStatus == "COMPLETED" {
			break
		}
		fmt.Println("Transcription job is still in progress...")
	}
}

// Get the transcription job result
func getTranscriptionJobResult(svc *transcribeservice.TranscribeService, jobName string) string {
	jobResult, err := svc.GetTranscriptionJob(&transcribeservice.GetTranscriptionJobInput{
		TranscriptionJobName: &jobName,
	})
	if err != nil {
		fmt.Println("Error retrieving transcription job result:", err)
		os.Exit(1)
	}

	var transcriptS3URL string
	if jobResult.TranscriptionJob.TranscriptionJobStatus != nil && *jobResult.TranscriptionJob.TranscriptionJobStatus == "COMPLETED" {
		// Fetch the transcript from the S3 URL provided in the job result
		transcriptS3URL = *jobResult.TranscriptionJob.Transcript.TranscriptFileUri
		fmt.Println("Transcript S3 URL:", transcriptS3URL)

		// Here, we can download the transcript file from the S3 URL and parse it as needed.
	} else {
		fmt.Println("Transcription job did not complete successfully.")
	}
	return transcriptS3URL
}

func getTranscriptFromS3(sess *session.Session, jobName, bucketName string) ([]byte, error) {
	svc := s3.New(sess)

	transcriptObject, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(jobName + ".json"), // Use the correct S3 object key
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transcript object: %v", err)
	}
	defer transcriptObject.Body.Close()

	var transcriptBytes bytes.Buffer
	_, err = io.Copy(&transcriptBytes, transcriptObject.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript object: %v", err)
	}

	return transcriptBytes.Bytes(), nil
}

// Extracts the video ID from the YouTube video URL
// func extractVideoID(url string) string {
// 	// Example YouTube URL: https://www.youtube.com/watch?v=VIDEO_ID
// 	parts := strings.Split(url, "v=")
// 	if len(parts) != 2 {
// 		fmt.Println("Invalid YouTube video URL")
// 		os.Exit(1)
// 	}
// 	return parts[1]
// }

// UploadTranscriptToS3 uploads the transcription JSON to the specified S3 bucket with the given object key
func uploadTranscriptToS3(sess *session.Session, bucketName, objectKey string, transcriptJSON []byte) error {
	svc := s3.New(sess)

	// Upload the transcript JSON to S3
	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(transcriptJSON),
	})
	if err != nil {
		return fmt.Errorf("failed to upload transcript to S3: %v", err)
	}

	return nil
}
