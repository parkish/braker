package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var extractTitleRegexp *regexp.Regexp = regexp.MustCompile(`^.+?title\s+(\d+)\:`)
var durationRegexp *regexp.Regexp = regexp.MustCompile(`^.+?duration: (\d+):(\d+):\d+$`)

type Track struct {
	id      string
	minutes int16
}

func getTrackInfo(dvdPath string, handbrakePath string) ([]Track, error) {
	output, err := runCommand(handbrakePath, "-i", dvdPath, "-t", "0")

	if err != nil {
		return nil, err
	}

	return parseTrackInfo(output)
}

func parseTrackInfo(output string) ([]Track, error) {
	var tracks []Track

	for _, line := range strings.Split(output, "\n") {
		matches := extractTitleRegexp.FindAllStringSubmatch(line, 1)

		if matches != nil {
			tracks = append(tracks, Track{id: matches[0][1]})
			continue
		}

		matches = durationRegexp.FindAllStringSubmatch(line, 1)

		if matches != nil {
			hoursStr := matches[0][1]
			hours, err := strconv.ParseInt(hoursStr, 10, 16)

			if err != nil {
				return nil, fmt.Errorf("Failed to parse minutes from line %s, hours %s", line, hoursStr)
			}

			minutesStr := matches[0][2]
			minutes, err := strconv.ParseInt(minutesStr, 10, 16)

			if err != nil {
				return nil, fmt.Errorf("Failed to parse minutes from line %s, minute %s", line, minutesStr)
			}

			tracks[len(tracks)-1].minutes = int16(minutes + (hours * 60))
		}
	}

	return tracks, nil
}

func extractTracks(dvdPath string, handbrakePath string, handbrakePreset string, tracks []Track) error {
	errorChan := make(chan error, len(tracks))
	wg := &sync.WaitGroup{}

	for _, track := range tracks {
		wg.Add(1)

		go func(track Track) {
			defer wg.Done()

			log.Printf("Extracting track %#v", track)

			outputFile := path.Join(os.Getenv("HOME"), "/Desktop", fmt.Sprintf("%s_%s_%s.mp4", handbrakePreset, path.Base(dvdPath), track.id))

			if _, err := os.Stat(outputFile); err == nil {
				log.Printf("Skipping track %#v - file already exists at %s", track, outputFile)
				return
			}

			log.Printf("Creating track %#v, %s", track, outputFile)

			videoTSFolder := path.Join(dvdPath, "VIDEO_TS")
			args := []string{handbrakePath, "-t", string(track.id), "-i", videoTSFolder, "--preset", handbrakePreset, "-o", outputFile}

			log.Printf("Command line: %s", strings.Join(args, " "))

			cmd := exec.Command(handbrakePath, "-t", track.id, "-i", videoTSFolder, "--preset", handbrakePreset, "-o", outputFile)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				errorChan <- err
				return
			}

			if _, err := os.Stat(outputFile); err != nil {
				errorChan <- fmt.Errorf("Failed to create track %s", outputFile)
				return
			}
		}(track)
	}

	log.Printf("Waiting for all track goroutines to finish...")
	wg.Wait()

	finalError := &bytes.Buffer{}

extract:
	for {
		select {
		case err := <-errorChan:
			finalError.WriteString(err.Error())
			finalError.WriteString("\n")
		default:
			break extract
		}
	}

	if finalError.Len() > 0 {
		return errors.New(finalError.String())
	}

	return nil
}

func runCommand(program string, args ...string) (string, error) {
	cmd := exec.Command(program, args...)

	rawData, err := cmd.CombinedOutput()

	if err != nil {
		return "", err
	}

	return string(rawData), nil
}

func main() {
	var dvdPath string
	var handbrakePath string
	var handbrakeProfile string

	flag.StringVar(&dvdPath, "dvdpath", "", "Path to the DVD folder")
	flag.StringVar(&handbrakePath, "handbrake", "", "Path to the HandBrakeCLI binary")
	flag.StringVar(&handbrakeProfile, "handbrakeprofile", "High Profile", "Handbrake preset profile")
	flag.Parse()

	if dvdPath == "" || handbrakePath == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	videoTSFolder := path.Join(dvdPath, "VIDEO_TS")

	if _, err := os.Stat(videoTSFolder); err != nil {
		log.Fatalf("Invalid DVD path : %s should exist", videoTSFolder)
	}

	// first get all the available tracks - sometimes the main feature isn't obvious
	tracks, err := getTrackInfo(dvdPath, handbrakePath)

	if err != nil {
		log.Fatalf("Failed to get track info : %s", err.Error())
	}

	if err := extractTracks(dvdPath, handbrakePath, handbrakeProfile, tracks); err != nil {
		log.Fatalln(err)
	}

	log.Printf("Conversion done")
}
