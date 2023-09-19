package etcdloganalyzer

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// subStringLevel defines a sub-string we'll scan pod log lines for, and the level the resulting
// interval should have. (Info, Warning, Error)
type subStringLevel struct {
	subString string
	level     monitorapi.IntervalLevel
}

type etcdLogLine struct {
	Level         string    `json:"level"`
	Timestamp     time.Time `json:"ts"`
	Msg           string    `json:"msg"`
	LocalMemberID string    `json:"local-member-id"`
}
