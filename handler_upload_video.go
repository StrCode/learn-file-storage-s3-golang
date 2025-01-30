package main

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	upload_limit := 1 << 30

	videoIdString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIdString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "videoID is not valid", nil)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "user is not permitted to work on video", err)
		return
	}

	r.ParseMultipartForm(int64(upload_limit))
	videoFile, fHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to process file", err)
		return
	}

	defer videoFile.Close()

	mediaType, _, mtErr := mime.ParseMediaType(fHeader.Header.Get("Content-Type"))
	if mtErr != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for video", nil)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "File must be in a jpeg or png format", nil)
		return
	}

	// file_ext := mediaTypeToExt(mediaType)

	tempFile, err := os.CreateTemp("/Users/vector/Downloads/", "*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "issues creating temp file", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, cpErr := io.Copy(tempFile, videoFile)
	if cpErr != nil {
		respondWithError(w, http.StatusInternalServerError, "cannot read file from the start", err)
		return
	}

	_, seekErr := tempFile.Seek(0, io.SeekStart)
	if seekErr != nil {
		respondWithError(w, http.StatusInternalServerError, "cannot read file from the start", err)
		return
	}

	fileKey := getAssetPath(mediaType)

	s3ObjectInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tempFile,
		ContentType: &mediaType,
	}

	_, dbErr := cfg.s3Client.PutObject(context.Background(), &s3ObjectInput)
	if dbErr != nil {
		respondWithError(w, http.StatusInternalServerError, "could not upload to AWS here", dbErr)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket, cfg.s3Region, fileKey)

	video.VideoURL = &videoURL

	viderr := cfg.db.UpdateVideo(video)
	if viderr != nil {
		respondWithError(w, http.StatusInternalServerError, "could not update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, struct{}{})
}
