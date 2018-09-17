package createconfig

import (
	"reflect"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestCreateConfig_GetVolumeMounts(t *testing.T) {
	data := spec.Mount{
		Destination: "/foobar",
		Type:        "bind",
		Source:      "foobar",
		Options:     []string{"ro", "rbind", "rprivate"},
	}
	config := CreateConfig{
		Volumes: []string{"foobar:/foobar:ro"},
	}
	specMount, err := config.GetVolumeMounts([]spec.Mount{})
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(data, specMount[0]))
}

func TestCreateConfig_GetTmpfsMounts(t *testing.T) {
	data := spec.Mount{
		Destination: "/homer",
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     []string{"rw", "size=787448k", "mode=1777"},
	}
	config := CreateConfig{
		Tmpfs: []string{"/homer:rw,size=787448k,mode=1777"},
	}
	tmpfsMount := config.GetTmpfsMounts()
	assert.True(t, reflect.DeepEqual(data, tmpfsMount[0]))

}
