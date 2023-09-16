package main

// import (
// 	"bytes"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/url"
// 	"os"
// 	"strings"
// 	"time"

// 	"github.com/aws/aws-sdk-go/aws"
// 	"github.com/aws/aws-sdk-go/service/transcribeservice"
// 	"github.com/kkdai/youtube/v2"
// 	"github.com/pepperstone-data/hermes/aws/s3"
// )

// const (
// 	//sample transcripts
// 	videoUrl1 = "https://www.youtube.com/watch?v=83X6Nu403Mk"
// 	videoUrl2 = "https://www.youtube.com/watch?v=xTuR7fBICeA"
// 	videoUrl3 = "https://www.youtube.com/watch?v=KFq3lbnsY_U"
// )

// type s3bucket struct {
// 	bucket s3.BucketOperator
// }

// func main() {
// 	transcriber(videoUrl1)
// 	transcriber(videoUrl2)
// 	transcriber(videoUrl3)
// }

// func transcriber(videoURL string) {
// 	videoID, _ := extractVideoIDFromURL(videoURL)
// 	bucketName := os.Getenv("AWS_BUCKET")

// 	bucketOptions := s3.Options{
// 		Region: os.Getenv("AWS_REGION"),
// 		Bucket: bucketName,
// 	}

// 	// S3 bucket for storing the YouTube video audio
// 	bucket, err := s3.New(bucketOptions)
// 	if err != nil {
// 		log.Fatalf("failed to create s3 bucket operator: %v", err)
// 	}

// 	s3bucket := s3bucket{
// 		bucket: bucket,
// 	}

// 	sess := s3bucket.bucket.GetService()
// 	// Creating an AWS Transcribe service client
// 	transcribeClient := transcribeservice.New(sess)

// 	audioFileURI, err := s3bucket.downloadYouTubeAudio(bucketName, videoURL)
// 	if err != nil {
// 		fmt.Println("Error downloading YouTube audio:", err)
// 		return
// 	}
// 	fmt.Println("YouTube audio downloaded and uploaded to S3:", audioFileURI)

// 	transcriptionJobName := "YouTubeTranscription-" + videoID + "-" + time.Now().Format("20060102150405")

// 	// Delete the existing transcription job with the same videoID
// 	deleteExistingTranscriptionJob(transcribeClient, transcriptionJobName)

// 	// Start a transcription job
// 	transcriptionJobResponse, err := transcribeClient.StartTranscriptionJob(&transcribeservice.StartTranscriptionJobInput{
// 		TranscriptionJobName: &transcriptionJobName,
// 		LanguageCode:         aws.String("en-US"), // Language is ENglish-US.
// 		Media: &transcribeservice.Media{
// 			MediaFileUri: aws.String(audioFileURI),
// 		},
// 	})
// 	if err != nil {
// 		fmt.Println("Error starting transcription job:", err)
// 		return
// 	}

// 	fmt.Println("Transcription Job Name:", *transcriptionJobResponse.TranscriptionJob.TranscriptionJobName)

// 	// Wait for the transcription job to complete
// 	waitForTranscriptionJob(transcribeClient, transcriptionJobName)

// 	// Get the transcription job result
// 	getTranscriptionJobResult(transcribeClient, transcriptionJobName)

// 	// Get the transcript from AWS Transcribe
// 	transcriptJSON, err := s3bucket.getTranscriptFromS3(transcriptionJobName, "aws-transcribe-us-east-1-prod")

// 	//AWS Internal S3 where transcripts are saved initially
// 	if err != nil {
// 		fmt.Println("Error getting transcript from S3:", err)
// 		return
// 	}

// 	// Upload the transcript to your S3 bucket
// 	transcriptObjectName := "youtube_video_transcript.json" // Object key for the transcript file in your S3 bucket
// 	err = s3bucket.uploadTranscriptToS3(bucketName, transcriptObjectName, transcriptJSON)
// 	if err != nil {
// 		fmt.Println("Error uploading transcript to S3:", err)
// 		return
// 	}

// 	fmt.Println("Transcript uploaded to S3:", "s3://"+bucketName+"/"+transcriptObjectName)

// }

// func extractVideoIDFromURL(youtubeURL string) (string, error) {
// 	parsedURL, err := url.Parse(youtubeURL)
// 	if err != nil {
// 		return "", err
// 	}

// 	queryParams, err := url.ParseQuery(parsedURL.RawQuery)
// 	if err != nil {
// 		return "", err
// 	}

// 	videoID := queryParams.Get("v")
// 	if videoID == "" {
// 		return "", fmt.Errorf("video ID not found in the URL")
// 	}

// 	return videoID, nil
// }

// func deleteExistingTranscriptionJob(svc *transcribeservice.TranscribeService, jobName string) {
// 	_, _ = svc.DeleteTranscriptionJob(&transcribeservice.DeleteTranscriptionJobInput{ //Ignore errors here
// 		TranscriptionJobName: &jobName,
// 	})
// }

// // DownloadYouTubeAudio downloads the audio from a YouTube video and uploads it to the specified S3 bucket
// func (s *s3bucket) downloadYouTubeAudio(bucketName, videoURL string) (string, error) {
// 	// Create a new YouTube client
// 	client := youtube.Client{}
// 	// Get the video ID from the YouTube video URL
// 	videoID, _ := extractVideoIDFromURL(videoURL)

// 	// Get the video info
// 	video, err := client.GetVideo(videoID)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get video info: %v", err)
// 	}

// 	// Find the audio format with the highest quality (preference: "audio/webm; codecs=opus", then "audio/webm", and finally "audio/mp4")
// 	var audioFormat *youtube.Format
// 	for _, format := range video.Formats {
// 		if strings.HasPrefix(format.MimeType, "audio/webm; codecs=opus") {
// 			audioFormat = &format
// 			break
// 		} else if strings.HasPrefix(format.MimeType, "audio/webm") && audioFormat == nil {
// 			audioFormat = &format
// 		} else if strings.HasPrefix(format.MimeType, "audio/mp4") && audioFormat == nil {
// 			audioFormat = &format
// 		}
// 	}

// 	if audioFormat == nil {
// 		return "", fmt.Errorf("no suitable audio format found")
// 	}

// 	// Download the audio stream
// 	stream, _, err := client.GetStream(video, audioFormat)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get audio stream: %v", err)
// 	}
// 	defer stream.Close()

// 	folderName := "extractedAudios"
// 	objectKey := folderName + "/" + videoID + ".mp3"

// 	err = s.bucket.Push(s3.File{
// 		MetaData: s3.MetaData{
// 			Filename: objectKey,
// 		},
// 		Content: stream,
// 	})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to upload audio to S3: %v", err)
// 	}

// 	audioFileURI := "s3://" + bucketName + "/" + objectKey
// 	return audioFileURI, nil
// }

// // UploadAudioToS3 uploads the audio stream to the specified S3 bucket with the given object key
// func (s *s3bucket) uploadAudioToS3(bucketName, objectKey string, audioStream io.Reader) error {
// 	file := s3.File{

// 		Content: audioStream,
// 	}
// 	err := s.bucket.Push(file)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// // Wait for the transcription job to complete
// func waitForTranscriptionJob(svc *transcribeservice.TranscribeService, jobName string) {
// 	for {
// 		jobStatus, err := svc.GetTranscriptionJob(&transcribeservice.GetTranscriptionJobInput{
// 			TranscriptionJobName: &jobName,
// 		})
// 		if err != nil {
// 			fmt.Println("Error retrieving transcription job status:", err)
// 			os.Exit(1)
// 		}
// 		if *jobStatus.TranscriptionJob.TranscriptionJobStatus == "COMPLETED" {
// 			break
// 		}
// 		fmt.Println("Transcription job is still in progress...")
// 	}
// }

// // Get the transcription job result
// func getTranscriptionJobResult(svc *transcribeservice.TranscribeService, jobName string) {
// 	jobResult, err := svc.GetTranscriptionJob(&transcribeservice.GetTranscriptionJobInput{
// 		TranscriptionJobName: &jobName,
// 	})
// 	if err != nil {
// 		fmt.Println("Error retrieving transcription job result:", err)
// 		os.Exit(1)
// 	}

// 	if jobResult.TranscriptionJob.TranscriptionJobStatus != nil && *jobResult.TranscriptionJob.TranscriptionJobStatus == "COMPLETED" {
// 		// Fetch the transcript from the S3 URL provided in the job result
// 		transcriptS3URL := *jobResult.TranscriptionJob.Transcript.TranscriptFileUri
// 		fmt.Println("Transcript S3 URL:", transcriptS3URL)
// 	} else {
// 		fmt.Println("Transcription job did not complete successfully.")
// 	}
// }
// func (s *s3bucket) getTranscriptFromS3(jobName, bucketName string) ([]byte, error) {
// 	data := s3.MetaData{
// 		Filename: jobName + ".json", // Provide the object key (filename) for the S3 object to download it.
// 	}
// 	s3file, err := s.bucket.Pull(data)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var obj []byte
// 	_, _ = s3file.Content.Read(obj)
// 	return obj, nil
// }

// // UploadTranscriptToS3 uploads the transcription JSON to the specified S3 bucket with the given object key
// func (s *s3bucket) uploadTranscriptToS3(bucketName, objectKey string, transcriptJSON []byte) error {
// 	file := s3.File{

// 		Content: bytes.NewReader(transcriptJSON),
// 	}
// 	err := s.bucket.Push(file)
// 	if err != nil {
// 		return fmt.Errorf("failed to upload transcript to S3: %v", err)
// 	}

// 	return nil
// }
