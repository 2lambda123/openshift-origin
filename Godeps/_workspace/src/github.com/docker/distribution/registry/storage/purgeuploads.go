package storage

import (
	"path"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	log "github.com/Sirupsen/logrus"
	storageDriver "github.com/docker/distribution/registry/storage/driver"
)

// uploadData stored the location of temporary files created during a layer upload
// along with the date the upload was started
type uploadData struct {
	containingDir string
	startedAt     time.Time
}

func newUploadData() uploadData {
	return uploadData{
		containingDir: "",
		// default to far in future to protect against missing startedat
		startedAt: time.Now().Add(time.Duration(10000 * time.Hour)),
	}
}

// PurgeUploads deletes files from the upload directory
// created before olderThan.  The list of files deleted and errors
// encountered are returned
func PurgeUploads(driver storageDriver.StorageDriver, olderThan time.Time, actuallyDelete bool) ([]string, []error) {
	log.Infof("PurgeUploads starting: olderThan=%s, actuallyDelete=%t", olderThan, actuallyDelete)
	uploadData, errors := getOutstandingUploads(driver)
	var deleted []string
	for _, uploadData := range uploadData {
		if uploadData.startedAt.Before(olderThan) {
			var err error
			log.Infof("Upload files in %s have older date (%s) than purge date (%s).  Removing upload directory.",
				uploadData.containingDir, uploadData.startedAt, olderThan)
			if actuallyDelete {
				err = driver.Delete(uploadData.containingDir)
			}
			if err == nil {
				deleted = append(deleted, uploadData.containingDir)
			} else {
				errors = append(errors, err)
			}
		}
	}

	log.Infof("Purge uploads finished.  Num deleted=%d, num errors=%d", len(deleted), len(errors))
	return deleted, errors
}

// getOutstandingUploads walks the upload directory, collecting files
// which could be eligible for deletion.  The only reliable way to
// classify the age of a file is with the date stored in the startedAt
// file, so gather files by UUID with a date from startedAt.
func getOutstandingUploads(driver storageDriver.StorageDriver) (map[string]uploadData, []error) {
	var errors []error
	uploads := make(map[string]uploadData, 0)

	inUploadDir := false
	root, err := defaultPathMapper.path(repositoriesRootPathSpec{})
	if err != nil {
		return uploads, append(errors, err)
	}
	err = Walk(driver, root, func(fileInfo storageDriver.FileInfo) error {
		filePath := fileInfo.Path()
		_, file := path.Split(filePath)
		if file[0] == '_' {
			// Reserved directory
			inUploadDir = (file == "_uploads")

			if fileInfo.IsDir() && !inUploadDir {
				return ErrSkipDir
			}

		}

		uuid, isContainingDir := uUIDFromPath(filePath)
		if uuid == "" {
			// Cannot reliably delete
			return nil
		}
		ud, ok := uploads[uuid]
		if !ok {
			ud = newUploadData()
		}
		if isContainingDir {
			ud.containingDir = filePath
		}
		if file == "startedat" {
			if t, err := readStartedAtFile(driver, filePath); err == nil {
				ud.startedAt = t
			} else {
				errors = pushError(errors, filePath, err)
			}

		}

		uploads[uuid] = ud
		return nil
	})

	if err != nil {
		errors = pushError(errors, root, err)
	}
	return uploads, errors
}

// uUIDFromPath extracts the upload UUID from a given path
// If the UUID is the last path component, this is the containing
// directory for all upload files
func uUIDFromPath(path string) (string, bool) {
	components := strings.Split(path, "/")
	for i := len(components) - 1; i >= 0; i-- {
		if uuid := uuid.Parse(components[i]); uuid != nil {
			return uuid.String(), i == len(components)-1
		}
	}
	return "", false
}

// readStartedAtFile reads the date from an upload's startedAtFile
func readStartedAtFile(driver storageDriver.StorageDriver, path string) (time.Time, error) {
	startedAtBytes, err := driver.GetContent(path)
	if err != nil {
		return time.Now(), err
	}
	startedAt, err := time.Parse(time.RFC3339, string(startedAtBytes))
	if err != nil {
		return time.Now(), err
	}
	return startedAt, nil
}
