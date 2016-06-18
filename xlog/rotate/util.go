package rotate

import (
	"regexp"
	"strings"
	"time"
)

var numRegex = regexp.MustCompile("[0-9]+")

func getLogName() string {
	txt := time.Now().String()[:26]
	txt = strings.Join(numRegex.FindAllString(txt, -1), "")
	return txt[:8] + "-" + txt[8:14] + "-" + txt[14:] + ".log"
}
