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

func reportStatus(id string, status string) {
	ModifyQueueItem(id, Processing, 0, status)
}

func EncodeFile(input string, id string, sizes string) error {
	reportStatus(id, "starting")
	reportStatus(id, "parsing_sizes")

	sizeList := strings.Split(sizes, ",")

	// check if all sizes are valid
	for _, s := range sizeList {
		reportStatus(id, "preparing_size:"+s)
		sm := getSizeMapping(s)
		if sm.Label == "" {
			return errors.New("invalid size: " + s)
		}
	}

	//if a folder with the same name already exists, check if it has a meta.json file. if not, delete the folder and start over
	metaPath := id + "/meta.json"
	reportStatus(id, "checking_existing")

	fileExists := storage.FileExists(metaPath)
	if fileExists {
		slog.Info("File already encoded, skipping", "id", id)
		reportStatus(id, "already_encoded")
		return nil
	}

	var masterPlaylist strings.Builder
	masterPlaylist.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")

	reportStatus(id, "creating_directories")
	storage.LocalDirectoryCreate("tmp/" + id)
	storage.DirectoryCreate(id)

	for _, s := range sizeList {
		sm := getSizeMapping(s)
		outputDir := "tmp/" + id + "/" + sm.Label
		reportStatus(id, "creating_output_dir:"+sm.Label)
		storage.LocalDirectoryCreate(outputDir)

		sm.Scale = strings.ReplaceAll(sm.Scale, ":", "x") // for master playlist

		hwaccel := os.Getenv("FFMPEG_HARDWARE_ACCEL")
		if hwaccel != "" {
			slog.Info("Using hardware acceleration", "hwaccel", hwaccel)
		} else {
			hwaccel = "none"
		}

		// First pass (bitrate analysis)
		reportStatus(id, "first_pass_ready:"+sm.Label)
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
				"passlogfile": fmt.Sprintf(storage.LocalStoragePath+"/%s/logfile", outputDir),
			}).OverWriteOutput()

		slog.Info("Encoding first pass", "resolution", sm.Label)
		reportStatus(id, "encoding_first_pass:"+sm.Label)
		if err := pass1.Run(); err != nil {
			slog.Error("Failed to encode first pass", "resolution", sm.Label, "error", err)
			reportStatus(id, "error_first_pass:"+sm.Label)
			return err
		}

		// Second pass (generate HLS)
		reportStatus(id, "second_pass_ready:"+sm.Label)
		pass2 := ffmpeg.Input(input, ffmpeg.KwArgs{
			"hwaccel": hwaccel,
		}).
			Output(fmt.Sprintf(storage.LocalStoragePath+"/%s/index.m3u8", outputDir), ffmpeg.KwArgs{
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
				"hls_segment_filename": fmt.Sprintf(storage.LocalStoragePath+"/%s/seg_%%03d.m4s", outputDir),
				"pass":                 "2",
				"passlogfile":          fmt.Sprintf(storage.LocalStoragePath+"/%s/logfile", outputDir),
			}).OverWriteOutput()

		slog.Info("Encoding second pass", "resolution", sm.Label)
		reportStatus(id, "encoding_second_pass:"+sm.Label)
		if err := pass2.Run(); err != nil {
			slog.Error("Failed to encode second pass", "resolution", sm.Label, "error", err)
			reportStatus(id, "error_second_pass:"+sm.Label)
			return err
		}

		// Move files from temp to final storage
		reportStatus(id, "moving_files:"+sm.Label)
		files, err := storage.LocalDirectoryListing(outputDir, false)
		if err != nil {
			reportStatus(id, "error_move_files:"+sm.Label)
			return err
		}
		for _, file := range files {
			src := outputDir + "/" + file
			dst := id + "/" + sm.Label + "/" + file
			file, err := storage.LocalFileGet(src)
			if err != nil {
				reportStatus(id, "error_file_get:"+sm.Label)
				return err
			}
			if err := storage.FilePut(dst, file); err != nil {
				reportStatus(id, "error_file_put:"+sm.Label)
				return err
			}
			storage.LocalFileDelete(src)
			slog.Debug("Moved file to final storage", "src", src, "dst", dst)
		}

		// Update master playlist
		masterPlaylist.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%s\n/%s\n",
			sm.VideoBitrate, sm.Scale, sm.Label,
		))
		reportStatus(id, "finished_size:"+sm.Label)
	}

	// Write master playlist
	reportStatus(id, "writing_master_playlist")
	storage.FilePut(id+"/master.m3u8", []byte(masterPlaylist.String()))

	//make imgs dir
	reportStatus(id, "creating_thumbnails")
	storage.LocalDirectoryCreate("tmp/" + id + "/imgs")

	slog.Info("Creating thumbnails and previews", "id", id)
	reportStatus(id, "generating_thumbnail")
	//make a thumbnail
	cmd := ffmpeg.Input(input).
		Output(storage.LocalStoragePath+"/tmp/"+id+"/imgs/thumbnail.jpg",
			ffmpeg.KwArgs{
				"vf":      "thumbnail,scale=1280:720",
				"vframes": "1",
			},
		).OverWriteOutput()

	err := cmd.Run()
	if err != nil {
		reportStatus(id, "error_thumbnail")
		return err
	}

	//move the thumbnail to the final storage
	reportStatus(id, "moving_thumbnail")
	file, err := storage.LocalFileGet("tmp/" + id + "/imgs/thumbnail.jpg")
	if err != nil {
		return err
	}
	if err := storage.FilePut(id+"/imgs/thumbnail.jpg", file); err != nil {
		return err
	}
	if err := storage.LocalFileDelete("tmp/" + id + "/imgs/thumbnail.jpg"); err != nil {
		return err
	}

	type Preview struct {
		Id           int
		W            int
		H            int
		Amount       int
		TileInterval int
	}

	var previews []Preview
	reportStatus(id, "generating_previews")
	cmd = ffmpeg.Input(input).
		Output(storage.LocalStoragePath+"/tmp/"+id+"/imgs/prev-%d.jpg",
			ffmpeg.KwArgs{
				"vf": "scale=160:90,fps=1/5,tile=25x1",
			},
		).OverWriteOutput()

	err = cmd.Run()
	if err != nil {
		reportStatus(id, "error_preview")
		return err
	}

	//read the sheets
	reportStatus(id, "listing_previews")
	sheets, err := storage.LocalDirectoryListing("tmp/"+id+"/imgs", false)
	if err != nil {
		reportStatus(id, "error_preview_listing")
		return err
	}

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
	reportStatus(id, "moving_previews")
	for _, preview := range previews {
		file, err := storage.LocalFileGet("tmp/" + id + "/imgs/prev-" + strconv.Itoa(preview.Id) + ".jpg")
		if err != nil {
			reportStatus(id, "error_preview_file_get")
			return err
		}
		if err := storage.FilePut(id+"/imgs/prev-"+strconv.Itoa(preview.Id)+".jpg", file); err != nil {
			reportStatus(id, "error_preview_file_put")
			return err
		}
		if err := storage.LocalFileDelete("tmp/" + id + "/imgs/prev-" + strconv.Itoa(preview.Id) + ".jpg"); err != nil {
			reportStatus(id, "error_preview_file_delete")
			return err
		}
	}

	//write previews
	reportStatus(id, "writing_preview_json")
	previewsJson, err := json.Marshal(previews)
	if err != nil {
		reportStatus(id, "error_preview_json")
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
	reportStatus(id, "writing_meta_json")
	metaJson, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		reportStatus(id, "error_meta_json")
		return err
	}
	storage.FilePut(metaPath, metaJson)
	slog.Info("Meta file written", "id", id)

	//remove tmp dir
	reportStatus(id, "cleanup")
	storage.LocalDirectoryDelete("tmp/" + id)
	reportStatus(id, "done")
	return nil
}
