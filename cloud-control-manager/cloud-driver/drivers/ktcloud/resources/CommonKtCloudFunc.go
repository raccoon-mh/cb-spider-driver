// Proof of Concepts of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// KT Cloud Driver PoC
//
// by ETRI, 2021.05.

package resources

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	// "reflect"
	// "crypto/aes"
	// "crypto/cipher"
	// "crypto/rand"
	// "io"
	"os"
	"os/exec"

	// "errors"
	// "encoding/hex"
	// "encoding/base64"
	// "hash"
	// "hash/fnv"
	// "github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"

	cblog "github.com/cloud-barista/cb-log"
	call "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/call-log"
)

var once sync.Once
var cblogger *logrus.Logger
var calllogger *logrus.Logger

//Cloud Object를 JSON String 타입으로 변환
func ConvertJsonString(v interface{}) (string, error) {
	jsonBytes, jsonErr := json.Marshal(v)
	if jsonErr != nil {
		cblogger.Error("JSON 변환 실패")
		cblogger.Error(jsonErr)
		return "", jsonErr
	}

	jsonString := string(jsonBytes)
	return jsonString, nil
}

// int32 to string 변환 : String(), int64 to string 변환 : strconv.Itoa()
func String(n int32) string {
	buf := [11]byte{}
	pos := len(buf)
	i := int64(n)
	signed := i < 0
	if signed {
		i = -i
	}
	for {
		pos--
		buf[pos], i = '0'+byte(i%10), i/10
		if i == 0 {
			if signed {
				pos--
				buf[pos] = '-'
			}
			return string(buf[pos:])
		}
	}
}

func InitLog() {
	once.Do(func() {
		// cblog is a global variable.
		cblogger = cblog.GetLogger("CB-SPIDER")
		calllogger = call.GetLogger("HISCALL")
	})
}

func LoggingError(hiscallInfo call.CLOUDLOGSCHEMA, err error) {
	cblogger.Error(err.Error())
	hiscallInfo.ErrorMSG = err.Error()
	calllogger.Error(call.String(hiscallInfo))
}

func LoggingInfo(hiscallInfo call.CLOUDLOGSCHEMA, start time.Time) {
	hiscallInfo.ElapsedTime = call.Elapsed(start)
	calllogger.Info(call.String(hiscallInfo))
}

func GetCallLogScheme(zone string, resourceType call.RES_TYPE, resourceName string, apiName string) call.CLOUDLOGSCHEMA {
	cblogger.Info(fmt.Sprintf("Call %s %s", call.KTCLOUD, apiName))

	return call.CLOUDLOGSCHEMA{
		CloudOS:      call.KTCLOUD,
		RegionZone:   zone,
		ResourceType: resourceType,
		ResourceName: resourceName,
		CloudOSAPI:   apiName,
	}
}

func logAndReturnError(callLogInfo call.CLOUDLOGSCHEMA, givenErrString string, v interface{}) (error) {
	newErr := fmt.Errorf(givenErrString + " %v", v)
	cblogger.Error(newErr.Error())
	LoggingError(callLogInfo, newErr)
	return newErr
}

func RunCommand(cmdName string, cmdArgs []string) (string, error) {
	/*
	Ref)
	var (
		cmdOut []byte
		cmdErr   error		
	)
	*/

	cblogger.Infof("cmdName : %s", cmdName)
	cblogger.Infof("cmdArgs : %s", cmdArgs)

	//if cmdOut, cmdErr = exec.Command(cmdName, cmdArgs...).Output(); cmdErr != nil {
	if cmdOut, cmdErr := exec.Command(cmdName, cmdArgs...).CombinedOutput(); cmdErr != nil {
		fmt.Fprintln(os.Stderr, "There was an error running command: ", cmdErr)
		//panic("Can't exec the command: " + cmdErr1.Error())
		fmt.Println(fmt.Sprint(cmdErr) + ": " + string(cmdOut))
		os.Exit(1)

		return string(cmdOut), cmdErr
	} else {
	fmt.Println("cmdOut : ", string(cmdOut))

	return string(cmdOut), nil
	}
}

func CheckFolder(folderPath string) error {
	// Check if the KeyPair Folder Exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
	}

	return nil
}

func CheckFolderAndCreate(folderPath string) error {
	// Check if the Folder Exists and Create it
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.Mkdir(folderPath, 0700); err != nil {
			return err
		}
	}
	return nil
}

func convertTimeFormat(inputTime string) (time.Time, error) {
	// Parse the input time using the given layout
	layout := "2006-01-02T15:04:05-0700"
	parsedTime, err := time.Parse(layout, inputTime)
	if err != nil {
		newErr := fmt.Errorf("Failed to Parse the Input Time Format!!")
		cblogger.Error(newErr.Error())
		return time.Time{}, newErr
	}
	return parsedTime, nil
}
