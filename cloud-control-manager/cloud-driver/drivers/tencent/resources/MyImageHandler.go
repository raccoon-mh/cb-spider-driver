package resources

import (
	"errors"
	"time"

	call "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/call-log"
	idrv "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces"
	irs "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/resources"
	"github.com/davecgh/go-spew/spew"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"

	//cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	cbs "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cbs/v20170312"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
)

//https://intl.cloud.tencent.com/document/product/213/4940
//https://intl.cloud.tencent.com/document/product/213/33276
//https://console.intl.cloud.tencent.com/api/explorer?Product=cvm&Version=2017-03-12&Action=DescribeImages&SignVersion=

// 비슷한 function 으로 ecm에 있는 서비스
//https://console.intl.cloud.tencent.com/api/explorer?Product=ecm&Version=2019-07-19&Action=CreateImage&SignVersion=

const (
	TENCENT_IMAGE_STATE_CREATING = "CREATING"
	TENCENT_IMAGE_STATE_NORMAL   = "NORMAL"
	TENCENT_IMAGE_STATE_USING    = "USING"

	TENCENT_IMAGE_STATE_ERROR = "Error"

	TENCENT_SNAPSHOT_STATE_NORMAL      = "NORMAL"
	TENCENT_SNAPSHOT_STATE_CREATING    = "CREATING"
	TENCENT_SNAPSHOT_STATE_ROLLBACKING = "ROLLBACKING"

	RESOURCE_TYPE_MYIMAGE = "image"
	IMAGE_TAG_DEFAULT     = "Name"
	IMAGE_TAG_SOURCE_VM   = "CB-VMSNAPSHOT-SOURCEVM-ID"
)

type TencentMyImageHandler struct {
	Region    idrv.RegionInfo
	Client    *cvm.Client
	CbsClient *cbs.Client
}

func (myImageHandler *TencentMyImageHandler) SnapshotVM(snapshotReqInfo irs.MyImageInfo) (irs.MyImageInfo, error) {

	hiscallInfo := GetCallLogScheme(myImageHandler.Region, call.MYIMAGE, snapshotReqInfo.IId.NameId, "SnapshotVM()")
	start := call.Start()

	existName, errExist := myImageHandler.myImageExist(snapshotReqInfo.IId.NameId)
	if errExist != nil {
		cblogger.Error(errExist)
		return irs.MyImageInfo{}, errExist
	}
	if existName {
		return irs.MyImageInfo{}, errors.New("A MyImage with the name " + snapshotReqInfo.IId.NameId + " already exists.")
	}

	vmRequest := cvm.NewDescribeInstancesRequest()
	request := cvm.NewCreateImageRequest()

	vmRequest.InstanceIds = common.StringPtrs([]string{snapshotReqInfo.SourceVM.SystemId})

	vmInfo, vmInfoErr := myImageHandler.Client.DescribeInstances(vmRequest)
	if vmInfoErr != nil {
		cblogger.Error(vmInfoErr)
		return irs.MyImageInfo{}, vmInfoErr
	}

	dataDiskSet := vmInfo.Response.InstanceSet[0].DataDisks
	var dataDiskIdList []string

	if len(dataDiskSet) > 0 {
		for _, dataDisk := range dataDiskSet {
			dataDiskId := dataDisk.DiskId
			dataDiskIdList = append(dataDiskIdList, *dataDiskId)
		}

		request.DataDiskIds = common.StringPtrs(dataDiskIdList)
	}

	//ImageName        *string `json:"ImageName,omitempty" name:"ImageName"`
	//InstanceId       *string `json:"InstanceId,omitempty" name:"InstanceId"`
	//ImageDescription *string `json:"ImageDescription,omitempty" name:"ImageDescription"`
	//ForcePoweroff    *string `json:"ForcePoweroff,omitempty" name:"ForcePoweroff"

	request.ImageName = common.StringPtr(snapshotReqInfo.IId.NameId)
	request.InstanceId = common.StringPtr(snapshotReqInfo.SourceVM.SystemId)

	// Tag 추가 ResourceType : instance(for CVM), host(for CDH), image(for image), keypair(for key)
	request.TagSpecification = []*cvm.TagSpecification{
		{
			ResourceType: common.StringPtr(RESOURCE_TYPE_MYIMAGE),
			Tags: []*cvm.Tag{
				{

					Key:   common.StringPtr(IMAGE_TAG_SOURCE_VM),
					Value: common.StringPtr(snapshotReqInfo.SourceVM.SystemId),
				},
			},
		},
	}

	// The returned "resp" is an instance of the CreateImageResponse class which corresponds to the request object
	response, err := myImageHandler.Client.CreateImage(request)
	hiscallInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		cblogger.Error(err)
		LoggingError(hiscallInfo, err)
		return irs.MyImageInfo{}, err
	}
	calllogger.Info(call.String(hiscallInfo))

	spew.Dump(response)

	myImageInfo, myImageErr := myImageHandler.GetMyImage(irs.IID{SystemId: *response.Response.ImageId})
	if myImageErr != nil {
		cblogger.Error(myImageErr)
		return irs.MyImageInfo{}, myImageErr
	}

	return myImageInfo, nil
}

/*
*
TODO : CommonHandlerm에 DescribeImages, DescribeImageById, DescribeImageStatus 추가할 것.
*/
func (myImageHandler *TencentMyImageHandler) ListMyImage() ([]*irs.MyImageInfo, error) {
	hiscallInfo := GetCallLogScheme(myImageHandler.Region, call.MYIMAGE, "MyImage", "ListMyImage()")
	start := call.Start()

	imageTypes := []string{}
	myImageSet, err := DescribeImages(myImageHandler.Client, nil, imageTypes)
	hiscallInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		cblogger.Error(err)
		LoggingError(hiscallInfo, err)
		return nil, err
	}
	calllogger.Info(call.String(hiscallInfo))

	myImageInfoList := []*irs.MyImageInfo{}

	for _, image := range myImageSet {
		myImageInfo, myImageInfoErr := convertImageSetToMyImageInfo(image)
		if myImageInfoErr != nil {
			continue
		}
		myImageInfoList = append(myImageInfoList, &myImageInfo)
	}
	return myImageInfoList, nil
}

func (myImageHandler *TencentMyImageHandler) GetMyImage(myImageIID irs.IID) (irs.MyImageInfo, error) {
	hiscallInfo := GetCallLogScheme(myImageHandler.Region, call.MYIMAGE, myImageIID.NameId, "GetMyImage()")
	start := call.Start()

	imageTypes := []string{"PRIVATE_IMAGE"}
	targetImage, err := DescribeImagesByID(myImageHandler.Client, myImageIID, imageTypes)
	hiscallInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		cblogger.Error(err)
		LoggingError(hiscallInfo, err)
		return irs.MyImageInfo{}, err
	}
	calllogger.Info(call.String(hiscallInfo))

	myImageInfo, myImageInfoErr := convertImageSetToMyImageInfo(&targetImage)
	if myImageInfoErr != nil {
		cblogger.Error(myImageInfoErr)
		return irs.MyImageInfo{}, myImageInfoErr
	}

	return myImageInfo, nil
}

/*
*
If the ImageState of an image is CREATING or USING, the image cannot be deleted. Call the DescribeImages API to query the image status.
Up to 10 custom images are allowed in each region. If you have run out of the quota, delete unused images to create new ones.
A shared image cannot be deleted.
*/
func (myImageHandler *TencentMyImageHandler) DeleteMyImage(myImageIID irs.IID) (bool, error) {
	hiscallInfo := GetCallLogScheme(myImageHandler.Region, call.MYIMAGE, myImageIID.NameId, "DeleteMyImage()")
	start := call.Start()

	// Image 상태 조회
	imageTypes := []string{"PRIVATE_IMAGE"}
	resultImg, err := DescribeImagesByID(myImageHandler.Client, myImageIID, imageTypes)
	if err != nil {
		return false, err
	}

	status := *resultImg.ImageState

	if status == TENCENT_IMAGE_STATE_CREATING || status == TENCENT_IMAGE_STATE_USING {
		return false, errors.New("CREATING or USING, the image cannot be deleted.")
	}

	// Snapshot 상태 조회
	snapshotIds := GetSnapshotIdsFromImage(resultImg)
	for _, snapshotId := range snapshotIds {
		snapshotStatus, err := DescribeSnapshotStatus(myImageHandler.CbsClient, irs.IID{SystemId: snapshotId})
		if err != nil {
			return false, err
		}

		if snapshotStatus != TENCENT_SNAPSHOT_STATE_NORMAL {
			return false, errors.New("CREATING or ROLLBACKING, the snapshot cannot be deleted.")
		}
	}

	// Image 삭제 처리
	request := cvm.NewDeleteImagesRequest()

	request.ImageIds = common.StringPtrs([]string{myImageIID.SystemId})
	// request.DeleteBindedSnap = common.BoolPtr(true)

	// The returned "resp" is an instance of the DeleteImagesResponse class which corresponds to the request object
	response, err := myImageHandler.Client.DeleteImages(request)
	hiscallInfo.ElapsedTime = call.Elapsed(start)

	if err != nil {
		cblogger.Error(err)
		LoggingError(hiscallInfo, err)
		return false, err
	}
	calllogger.Info(call.String(hiscallInfo))

	requestId := response.Response.RequestId
	cblogger.Info("requestId : %s", requestId)

	// Image 삭제 대기
	_, err = WaitForDelete(myImageHandler.Client, myImageIID)
	if err != nil {
		cblogger.Error(err)
		return false, err
	}

	// Snapshot 삭제 처리
	_, snapshotErr := myImageHandler.DeleteSnapshotById(snapshotIds)
	if snapshotErr != nil {
		cblogger.Error(snapshotErr)
	}

	return true, nil
}

func convertImageSetToMyImageInfo(tencentImage *cvm.Image) (irs.MyImageInfo, error) {
	returnMyImageInfo := irs.MyImageInfo{}

	returnMyImageInfo.IId = irs.IID{NameId: *tencentImage.ImageName, SystemId: *tencentImage.ImageId}
	returnMyImageInfo.SourceVM = irs.IID{SystemId: *tencentImage.Tags[0].Value}
	returnMyImageInfo.CreatedTime, _ = time.Parse(time.RFC3339, *tencentImage.CreatedTime)
	returnMyImageInfo.Status = convertTenStatusToImageStatus(*tencentImage.ImageState)
	return returnMyImageInfo, nil
}

func convertTenStatusToImageStatus(status string) irs.MyImageStatus {
	var returnStatus irs.MyImageStatus

	// CREATING / NORMAL / CREATEFAILED / USING / SYNCING / IMPORTING / IMPORTFAILED
	if status == TENCENT_IMAGE_STATE_NORMAL {
		returnStatus = irs.MyImageAvailable
	} else {
		returnStatus = irs.MyImageUnavailable
	}

	return returnStatus
}

// Image에 대한 snap 삭제
func (myImageHandler *TencentMyImageHandler) DeleteSnapshotById(snapshotIds []string) (bool, error) {
	request := cbs.NewDeleteSnapshotsRequest()
	request.SnapshotIds = common.StringPtrs(snapshotIds)
	request.DeleteBindImages = common.BoolPtr(true)

	DiskHandler := TencentDiskHandler{
		Region: myImageHandler.Region,
		Client: myImageHandler.CbsClient,
	}

	response, err := DiskHandler.Client.DeleteSnapshots(request)

	if err != nil {
		cblogger.Error(err)
		return false, err
	}

	requestId := response.Response.RequestId
	cblogger.Info("requestId : %s", requestId)

	return true, nil
}

/*
myimage가 존재하는지 check
동일이름이 없으면 false, 있으면 true
*/
func (myImageHandler *TencentMyImageHandler) myImageExist(chkName string) (bool, error) {
	cblogger.Debugf("chkName : %s", chkName)

	request := cvm.NewDescribeImagesRequest()

	request.Filters = []*cvm.Filter{
		{
			Name:   common.StringPtr("image-name"),
			Values: common.StringPtrs([]string{chkName}),
		},
	}

	response, err := myImageHandler.Client.DescribeImages(request)
	if err != nil {
		cblogger.Error(err)
		return false, err
	}

	if *response.Response.TotalCount < 1 {
		return false, nil
	}

	cblogger.Infof("MyImage 정보 찾음 - MyImageId:[%s] / MyImageName:[%s]", *response.Response.ImageSet[0].ImageId, *response.Response.ImageSet[0].ImageName)
	return true, nil
}

// https://console.tencentcloud.com/api/explorer?Product=cvm&Version=2017-03-12&Action=DescribeImages
// Window OS 여부
// imageType : MyImage는 PRIVATE,    PRIVATE_IMAGE, PUBLIC_IMAGE, SHARED_IMAGE
func (myImageHandler *TencentMyImageHandler) CheckWindowsImage(myImageIID irs.IID) (bool, error) {
	hiscallInfo := GetCallLogScheme(myImageHandler.Region, call.MYIMAGE, myImageIID.NameId, "CheckWindowsImage()")
	start := call.Start()

	imageTypes := []string{"PRIVATE_IMAGE"}
	isWindow := false

	resultImg, err := DescribeImagesByID(myImageHandler.Client, myImageIID, imageTypes)
	hiscallInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		cblogger.Error(err)
		LoggingError(hiscallInfo, err)
		return isWindow, err
	}
	calllogger.Info(call.String(hiscallInfo))

	platform := GetOsType(resultImg)
	if platform == "Windows" {
		isWindow = true
	}

	return false, nil

}
