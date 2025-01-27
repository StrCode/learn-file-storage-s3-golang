package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, fHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No file was detected", err)
		return
	}

	defer file.Close()

	fileMediaType := fHeader.Header.Get("Content-Type")

	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not decode file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "could not find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "you do not have permission", err)
		return
	}

	videoThumbnail := thumbnail{
		data,
		fileMediaType,
	}

	videoThumbnails[videoID] = videoThumbnail

	thumbnailStr := fmt.Sprintf("http//localhost:%s/api/thumbnails/%s", cfg.port, video.ID.String())
	fmt.Println(thumbnailStr)

	video.ThumbnailURL = &thumbnailStr
	video.UpdatedAt = time.Now()

	dbErr := cfg.db.UpdateVideo(video)
	if dbErr != nil {
		respondWithError(w, http.StatusInternalServerError, "could not update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
