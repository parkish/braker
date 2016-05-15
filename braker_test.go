package main

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func Test_parseTrackInfo(t *testing.T) {
	output, err := ioutil.ReadFile("testdata/HandbrakeTrackInfoOutput.txt")
	assert.NoError(t, err)

	actualTracks, err := parseTrackInfo(string(output))
	assert.NoError(t, err)

	expectedTracks := []Track{
		Track{id: "2", minutes: 1},
		Track{id: "4", minutes: 26},
		Track{id: "5", minutes: 26},
		Track{id: "6", minutes: 26},
		Track{id: "7", minutes: 26},
	}

	assert.EqualValues(t, expectedTracks, actualTracks)
}
