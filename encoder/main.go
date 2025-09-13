package encoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"goenc/storage"
	"log/slog"
	"os"
	"strconv"
	"strings"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

//TODO: pass local storage location to ffmpeg

type SizeMappingType struct {
	Label        string
	Scale        string
	VideoBitrate string
	AudioBitrate string
	Bufsize      string
	Crf          string
}

var SizeMapping []SizeMappingType = []SizeMappingType{
	{"2160p", "3840:2160", "12000k", "192k", "18000k", "18"},
	{"1440p", "2560:1440", "8000k", "160k", "12000k", "19"},
	{"1080p", "1920:1080", "5000k", "160k", "8000k", "20"},
	{"720p", "1280:720", "2500k", "128k", "4000k", "22"},
	{"480p", "854:480", "1200k", "96k", "2000k", "23"},
	{"360p", "640:360", "800k", "96k", "1500k", "24"},
	{"240p", "426:240", "500k", "64k", "1000k", "25"},
	{"144p", "256:144", "300k", "64k", "600k", "26"},
}

func getSizeMapping(size string) SizeMappingType {
	for _, sm := range SizeMapping {
		if sm.Label == size {
			return sm
		}
	}
	return SizeMappingType{}
}

func EncodeFile(input string, id string, sizes string) error {

	sizeList := strings.Split(sizes, ",")

	// check if all sizes are valid
	for _, s := range sizeList {
		sm := getSizeMapping(s)
		if sm.Label == "" {
			return errors.New("invalid size: " + s)
		}
	}

	//if a folder with the same name already exists, check if it has a meta.json file. if not, delete the folder and start over
	metaPath := id + "/meta.json"
	if storage.FileExists(metaPath) {
		slog.Info("File already encoded, skipping", "id", id)
		return nil
	}

	var masterPlaylist strings.Builder
	masterPlaylist.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")

	storage.LocalDirectoryCreate("tmp/" + id)
	storage.DirectoryCreate(id)

	for _, s := range sizeList {
		sm := getSizeMapping(s)
		outputDir := "tmp/" + id + "/" + sm.Label
		storage.LocalDirectoryCreate(outputDir)

		sm.Scale = strings.ReplaceAll(sm.Scale, ":", "x") // for master playlist

		hwaccel := os.Getenv("FFMPEG_HARDWARE_ACCEL")
		if hwaccel != "" {
			slog.Info("Using hardware acceleration", "hwaccel", hwaccel)
		} else {
			hwaccel = "none"
		}

		// First pass (bitrate analysis)
		pass1 := ffmpeg.Input(input, ffmpeg.KwArgs{
			"hwaccel": hwaccel,
		}).
			Output("/dev/null", ffmpeg.KwArgs{
				"c:v":     "libx264",
				"preset":  "slow",
				"b:v":     sm.VideoBitrate, // bitrate mode
				"maxrate": sm.VideoBitrate,
				"bufsize": sm.Bufsize,
				"vf": fmt.Sprintf(
					"scale=w=%s:h=%s:force_original_aspect_ratio=decrease:force_divisible_by=2",
					strings.Split(sm.Scale, "x")[0],
					strings.Split(sm.Scale, "x")[1],
				),
				"c:a":         "aac",
				"b:a":         sm.AudioBitrate,
				"pass":        "1",
				"an":          "",    // disable audio for first pass
				"f":           "mp4", // required for /dev/null replacement
				"passlogfile": fmt.Sprintf("data/%s/logfile", outputDir),
			}).OverWriteOutput()

		slog.Info("Encoding first pass", "resolution", sm.Label)
		if err := pass1.Run(); err != nil {
			slog.Error("Failed to encode first pass", "resolution", sm.Label, "error", err)
			return err
		}

		// Second pass (generate HLS)
		pass2 := ffmpeg.Input(input, ffmpeg.KwArgs{
			"hwaccel": hwaccel,
		}).
			Output(fmt.Sprintf("data/%s/index.m3u8", outputDir), ffmpeg.KwArgs{
				"c:v":     "libx264",
				"preset":  "slow",
				"b:v":     sm.VideoBitrate, // bitrate mode
				"maxrate": sm.VideoBitrate,
				"bufsize": sm.Bufsize,
				"vf": fmt.Sprintf(
					"scale=w=%s:h=%s:force_original_aspect_ratio=decrease:force_divisible_by=2",
					strings.Split(sm.Scale, "x")[0],
					strings.Split(sm.Scale, "x")[1],
				),
				"c:a":                  "aac",
				"b:a":                  sm.AudioBitrate,
				"hls_time":             "4",
				"hls_playlist_type":    "vod",
				"hls_segment_type":     "fmp4",
				"hls_segment_filename": fmt.Sprintf("data/%s/seg_%%03d.m4s", outputDir),
				"pass":                 "2",
				"passlogfile":          fmt.Sprintf("data/%s/logfile", outputDir),
			}).OverWriteOutput()

		slog.Info("Encoding second pass", "resolution", sm.Label)
		if err := pass2.Run(); err != nil {
			slog.Error("Failed to encode second pass", "resolution", sm.Label, "error", err)
			return err
		}

		// Move files from temp to final storage
		files := storage.LocalDirectoryListing(outputDir)
		for _, file := range files {
			src := outputDir + "/" + file
			dst := id + "/" + sm.Label + "/" + file
			storage.FilePut(dst, storage.LocalFileGet(src))
			storage.LocalFileDelete(src)
			slog.Debug("Moved file to final storage", "src", src, "dst", dst)
		}

		// Update master playlist
		masterPlaylist.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%s\n/%s\n",
			sm.VideoBitrate, sm.Scale, sm.Label,
		))
	}

	// Write master playlist
	storage.FilePut(id+"/master.m3u8", []byte(masterPlaylist.String()))

	//make imgs dir
	storage.LocalDirectoryCreate("tmp/" + id + "/imgs")

	slog.Info("Creating thumbnails and previews", "id", id)
	//make a thumbnail
	cmd := ffmpeg.Input(input).
		Output("data/tmp/"+id+"/imgs/thumbnail.jpg",
			ffmpeg.KwArgs{
				"vf":      "thumbnail,scale=1280:720",
				"vframes": "1",
			},
		).OverWriteOutput()

	err := cmd.Run()
	if err != nil {
		return err
	}

	//move the thumbnail to the final storage
	storage.FilePut(id+"/imgs/thumbnail.jpg", storage.LocalFileGet("tmp/"+id+"/imgs/thumbnail.jpg"))
	storage.LocalFileDelete("tmp/" + id + "/imgs/thumbnail.jpg")

	type Preview struct {
		Id           int
		W            int
		H            int
		Amount       int
		TileInterval int
	}

	var previews []Preview
	cmd = ffmpeg.Input(input).
		Output("data/tmp/"+id+"/imgs/prev-%d.jpg",
			ffmpeg.KwArgs{
				"vf": "scale=160:90,fps=1/5,tile=25x1",
			},
		).OverWriteOutput()

	err = cmd.Run()
	if err != nil {
		return err
	}

	//read the sheets
	sheets := storage.LocalDirectoryListing("tmp/" + id + "/imgs")

	for _, sheet := range sheets {
		if !strings.HasPrefix(sheet, "prev-") {
			continue
		}

		// Extract the index from the filename, e.g., "prev-0.jpg"
		var idx int
		fmt.Sscanf(sheet, "prev-%d.jpg", &idx)
		previews = append(previews, Preview{
			Id:           idx,
			W:            160,
			H:            90,
			Amount:       25,
			TileInterval: 5,
		})
	}

	//move the previews to the final storage
	for _, preview := range previews {
		storage.FilePut(id+"/imgs/prev-"+strconv.Itoa(preview.Id)+".jpg", storage.LocalFileGet("tmp/"+id+"/imgs/prev-"+strconv.Itoa(preview.Id)+".jpg"))
		storage.LocalFileDelete("tmp/" + id + "/imgs/prev-" + strconv.Itoa(preview.Id) + ".jpg")
	}

	//write previews
	previewsJson, err := json.Marshal(previews)
	if err != nil {
		return err
	}
	storage.FilePut(id+"/imgs/preview.json", previewsJson)

	slog.Info("Thumbnails and previews done", "id", id)
	meta := struct {
		ID    string   `json:"id"`
		Sizes []string `json:"sizes"`
		File  string   `json:"file"`
	}{
		ID:    id,
		Sizes: sizeList,
		File:  input,
	}
	metaJson, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	storage.FilePut(metaPath, metaJson)
	slog.Info("Meta file written", "id", id)

	//remove tmp dir
	storage.LocalDirectoryDelete("tmp/" + id)

	return nil
}
