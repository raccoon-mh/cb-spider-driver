// Proof of Concepts of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// This is a Cloud Driver Example for PoC Test.
//
// by ETRI, Innogrid, 2021.12.
// by ETRI, 2022.04.

package resources

import (
	// "errors"
	"fmt"
	"errors"
	"strconv"
	"strings"
	// "github.com/davecgh/go-spew/spew"

	nhnsdk "github.com/cloud-barista/nhncloud-sdk-go"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/extensions/secgroups"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/networking/v2/extensions/security/rules"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/networking/v2/extensions/external"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/networking/v2/networks"

	call "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/call-log"
	idrv "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces"
	irs "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/resources"
)

type NhnCloudSecurityHandler struct {
	RegionInfo    idrv.RegionInfo
	VMClient      *nhnsdk.ServiceClient
	NetworkClient *nhnsdk.ServiceClient
}

func (securityHandler *NhnCloudSecurityHandler) CreateSecurity(securityReqInfo irs.SecurityReqInfo) (irs.SecurityInfo, error) {
	cblogger.Info("NHN Cloud Cloud Driver: called CreateSecurity()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, securityReqInfo.IId.NameId, "CreateSecurity()")

	// Check if the SecurityGroup Exists
	sgInfoList, err := securityHandler.ListSecurity()
	if err != nil {
		newErr := fmt.Errorf("Failed to Get SG List!! : [%v] ", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	for _, sgInfo := range sgInfoList {
		if sgInfo.IId.NameId == securityReqInfo.IId.NameId {
			newErr := fmt.Errorf("Security Group with name [%s] exists already!!", securityReqInfo.IId.NameId)
			cblogger.Error(newErr.Error())
			LoggingError(callLogInfo, newErr)
			return irs.SecurityInfo{}, newErr
		}
	}

	// Create SecurityGroup
	createOpts := secgroups.CreateOpts{
		Name:        securityReqInfo.IId.NameId,
		Description: securityReqInfo.IId.NameId,
	}
	start := call.Start()
	newSG, err := secgroups.Create(securityHandler.VMClient, createOpts).Extract()
	if err != nil {
		newErr := fmt.Errorf("Failed to Create New S/G on NHNCLOUD!! : [%v] ", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	} else {
		cblogger.Infof("Succeeded in Creating New S/G : [%s]", securityReqInfo.IId.NameId)
	}
	LoggingInfo(callLogInfo, start)
	cblogger.Infof("New S/G SystemId : [%s]", newSG.ID)

	newSGIID := irs.IID {
		SystemId: newSG.ID,
	}

	// Add Requested S/G Rules to the New S/G
	_, addErr := securityHandler.AddRules(newSGIID, securityReqInfo.SecurityRules)
	if err != nil {
		newErr := fmt.Errorf("Failed to Add Rule on the S/G!! : [%v] ", addErr)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	// Basically, Open 'Outbound' All Protocol for Any S/G (<= CB-Spider Rule)
	openErr := securityHandler.OpenOutboundAllProtocol(newSGIID)
	if openErr != nil {
		cblogger.Error(openErr)
		LoggingError(callLogInfo, openErr)
		// return irs.SecurityInfo{}, openErr
	}

	// Return Created S/G Info.
	newSGInfo, getErr := securityHandler.GetSecurity(newSGIID)
	if err != nil {
		newErr := fmt.Errorf("Failed to Get New S/G info!! : [%v] ", getErr)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	return newSGInfo, nil
}

func (securityHandler *NhnCloudSecurityHandler) ListSecurity() ([]*irs.SecurityInfo, error) {
	cblogger.Info("NHN Cloud Cloud Driver: called ListSecurity()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, "ListSecurity()", "ListSecurity()")

	// Get Security Group list
	start := call.Start()
	allPages, err := secgroups.List(securityHandler.VMClient).AllPages()
	if err != nil {
		newErr := fmt.Errorf("Failed to Get SG List from NhnCloud!! : [%v] ", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return nil, newErr
	}

	nhnSGList, err := secgroups.ExtractSecurityGroups(allPages)
	if err != nil {
		newErr := fmt.Errorf("Failed to Extract SG List from NhnCloud!! : [%v] ", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return nil, newErr
	}
	LoggingInfo(callLogInfo, start)


	// Get the Default VPC SystemID
	vpcSystemId, err := securityHandler.GetDefaultVPCSystemID()
	if err != nil {
		newErr := fmt.Errorf("Failed to Get Get the Default VPC SystemID : [%v]", err)
		cblogger.Error(newErr.Error())
		return nil, newErr
	}

	// Mapping S/G list info.
	var sgInfoList []*irs.SecurityInfo
	for _, nhnSG := range nhnSGList {
		sgInfo, err := securityHandler.MappingSecurityInfo(nhnSG, vpcSystemId)
		if err != nil {
			cblogger.Error(err.Error())
			LoggingError(callLogInfo, err)
			return nil, err
		}
		sgInfoList = append(sgInfoList, sgInfo)
	}	
	return sgInfoList, nil
}

func (securityHandler *NhnCloudSecurityHandler) GetSecurity(securityIID irs.IID) (irs.SecurityInfo, error) {
	cblogger.Info("NHN Cloud Cloud Driver: called GetSecurity()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, securityIID.SystemId, "GetSecurity()")

	start := call.Start()
	nhnSG, err := secgroups.Get(securityHandler.VMClient, securityIID.SystemId).Extract()
	if err != nil {
		newErr := fmt.Errorf("Failed to Get the S/G info from NHNCLOUD!! : [%v] ", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}
	LoggingInfo(callLogInfo, start)
	// spew.Dump(nhnSG)	

	// Get the Default VPC SystemID
	vpcSystemId, err := securityHandler.GetDefaultVPCSystemID()
	if err != nil {
		newErr := fmt.Errorf("Failed to Get Get the Default VPC SystemID : [%v]", err)
		cblogger.Error(newErr.Error())
		return irs.SecurityInfo{}, newErr
	}

	securityInfo, err := securityHandler.MappingSecurityInfo(*nhnSG, vpcSystemId)
	if err != nil {
		cblogger.Error(err.Error())
		LoggingError(callLogInfo, err)
		return irs.SecurityInfo{}, err
	}
	return *securityInfo, nil
}

func (securityHandler *NhnCloudSecurityHandler) DeleteSecurity(securityIID irs.IID) (bool, error) {
	cblogger.Info("NHN Cloud Cloud Driver: called DeleteSecurity()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, securityIID.SystemId, "DeleteSecurity()")

	start := call.Start()
	result := secgroups.Delete(securityHandler.VMClient, securityIID.SystemId)
	if result.Err != nil {
		newErr := fmt.Errorf("Failed to Delete the S/G on NHNCLOUD!! : [%v] ", result.Err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return false, newErr
	}
	LoggingInfo(callLogInfo, start)

	return true, nil
}

func (securityHandler *NhnCloudSecurityHandler) AddRules(sgIID irs.IID, securityRules *[]irs.SecurityRuleInfo) (irs.SecurityInfo, error) {
	cblogger.Info("NHN Cloud Driver: called AddRules()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, sgIID.SystemId, "AddRules()")

	if sgIID.SystemId == "" {
		newErr := fmt.Errorf("Invalid S/G SystemId!!")
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	// Check if the S/G exists
	sgInfo, err := securityHandler.GetSecurity(sgIID)
	if err != nil {
		newErr := fmt.Errorf("Failed to Find any S/G info. with the SystemId : [%s] : [%v]", sgIID.SystemId, err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	cblogger.Infof("S/G SystemId to Add the Rules [%s]", sgInfo.IId.NameId)

	// Add SecurityGroup Rules to the S/G
	for _, curRule := range *securityRules {
		if curRule.Direction == "" {
			return irs.SecurityInfo{}, errors.New("Failed to Find 'Direction' Value in the requested rule!!")
		} else if curRule.IPProtocol == "" {
			return irs.SecurityInfo{}, errors.New("Failed to Find 'IPProtocol' Value in the requested rule!!")
		} else if curRule.FromPort == "" {
			return irs.SecurityInfo{}, errors.New("Failed to Find 'FromPort' Value in the requested rule!!")
		} else if curRule.ToPort == "" {
			return irs.SecurityInfo{}, errors.New("Failed to Find 'ToPort' Value in the requested rule!!")
		} else if curRule.CIDR == "" {
			return irs.SecurityInfo{}, errors.New("Failed to Find 'CIDR' Value in the requested rule!!")
		}

		cblogger.Infof("curRule.IPProtocol : [%s]", curRule.IPProtocol)
		
		if strings.EqualFold(curRule.IPProtocol, "ALL") { 	// Add SecurityGroup Rules in case of 'All Traffic Open Rule'
			if strings.EqualFold(curRule.FromPort, "-1") && strings.EqualFold(curRule.ToPort, "-1") {
				var direction string
				if strings.EqualFold(curRule.Direction, "inbound") {
					direction = string(rules.DirIngress)
				} else if strings.EqualFold(curRule.Direction, "outbound") {
					direction = string(rules.DirEgress)
				} else {
					return irs.SecurityInfo{}, errors.New("Invalid Rule Direction!!")
				}

				allProtocolTypeCode := []string {"tcp", "udp", "icmp"}
				allCIDR := "0.0.0.0/0"

				for _, curProtocolType := range allProtocolTypeCode {
					var createRuleOpts rules.CreateOpts		
					if strings.EqualFold(curProtocolType, "icmp") {  // Without fromPort / toPort
						createRuleOpts = rules.CreateOpts{
							Direction:      rules.RuleDirection(direction),
							EtherType:      rules.EtherType4,
							SecGroupID:     sgIID.SystemId,
							Protocol:       rules.RuleProtocol(curProtocolType),  //Caution!!
							RemoteIPPrefix: allCIDR, //Caution!!
						}
					} else {
						var fromPort int
						var toPort int
						if strings.EqualFold(curRule.FromPort, "-1") && strings.EqualFold(curRule.ToPort, "-1") {  // Check again
							fromPort = 1
							toPort = 65535
						} 

						createRuleOpts = rules.CreateOpts{
							Direction:      rules.RuleDirection(direction),
							EtherType:      rules.EtherType4,
							SecGroupID:     sgIID.SystemId,
							PortRangeMin:   fromPort,
							PortRangeMax:   toPort,
							Protocol:       rules.RuleProtocol(curProtocolType),  //Caution!!
							RemoteIPPrefix: allCIDR, //Caution!!
						}
					}
		
					start := call.Start()
					_, err := rules.Create(securityHandler.NetworkClient, createRuleOpts).Extract()
					if err != nil {
						newErr := fmt.Errorf("Failed to Create New Rule to the S/G : [%s] : [%v]", sgIID.SystemId, err)
						cblogger.Error(newErr.Error())
						LoggingError(callLogInfo, newErr)
						return irs.SecurityInfo{}, newErr
					} 		
					LoggingInfo(callLogInfo, start)
					cblogger.Infof("Succeeded in Adding New [%s], [%s] Rule!!", curRule.Direction, curProtocolType)		
				}
			} else {
				return irs.SecurityInfo{}, errors.New("To Specify 'All Traffic Allow Rule', Specify '-1' as FromPort/ToPort!!")
			}
		} else {
			// Add SecurityGroup Rules if not 'All Traffic Open Rule'
			var direction string
			if strings.EqualFold(curRule.Direction, "inbound") {
				direction = string(rules.DirIngress)
			} else if strings.EqualFold(curRule.Direction, "outbound") {
				direction = string(rules.DirEgress)
			} else {
				return irs.SecurityInfo{}, errors.New("Invalid Rule Direction!!")
			}

			var createRuleOpts rules.CreateOpts

			if strings.EqualFold(curRule.IPProtocol, "icmp") {
				createRuleOpts = rules.CreateOpts{
					Direction:      rules.RuleDirection(direction),
					EtherType:      rules.EtherType4,
					SecGroupID:     sgIID.SystemId,
					Protocol:       rules.RuleProtocol(strings.ToLower(curRule.IPProtocol)),
					RemoteIPPrefix: curRule.CIDR,
				}
			} else {
				var fromPort int
				var toPort int
				if (curRule.FromPort == "-1") || (curRule.ToPort == "-1") {
					fromPort = 1
					toPort = 65535
				} else {
					fromPort, _ = strconv.Atoi(curRule.FromPort)
					toPort, _ = strconv.Atoi(curRule.ToPort)
				}

				createRuleOpts = rules.CreateOpts{
					Direction:      rules.RuleDirection(direction),
					EtherType:      rules.EtherType4,
					SecGroupID:     sgIID.SystemId,
					PortRangeMin:   fromPort,
					PortRangeMax:   toPort,
					Protocol:       rules.RuleProtocol(strings.ToLower(curRule.IPProtocol)),
					RemoteIPPrefix: curRule.CIDR,
				}
			}

			start := call.Start()
			_, err := rules.Create(securityHandler.NetworkClient, createRuleOpts).Extract()
			if err != nil {
				newErr := fmt.Errorf("Failed to Create New Rule to the S/G : [%s] : [%v]", sgIID.SystemId, err)
				cblogger.Error(newErr.Error())
				LoggingError(callLogInfo, newErr)
				return irs.SecurityInfo{}, newErr
			}
			LoggingInfo(callLogInfo, start)
			// Note : OpenStack Bug : Sometimes this function makes an error (After adding a rule successfully ) like : "Security group rule already exists. Rule id is ~~~~~~~."
			// Ref) https://bugzilla.redhat.com/show_bug.cgi?id=1786675
			cblogger.Infof("Succeeded in Adding New [%s], [%s] Rule!!", curRule.Direction, rules.RuleProtocol(strings.ToLower(curRule.IPProtocol)))		
		}
	}

	// Return Current SecurityGroup Info.
	securityInfo, err := securityHandler.GetSecurity(sgIID)
	if err != nil {
		newErr := fmt.Errorf("Failed to Get the S/G Info : [%s] : [%v]", sgIID.SystemId, err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return irs.SecurityInfo{}, newErr
	}

	// // AddServer will associate a server and a security group, enforcing the rules of the group on the server.		
	// addServerResult := secgroups.AddServer(securityHandler.VMClient, serverID, securityIID.NameId)

	// // RemoveServer will disassociate a server from a security grou
	// removeServerResult := secgroups.RemoveServer(securityHandler.VMClient, serverID, securityIID.NameId)

	return securityInfo, nil
}

func (securityHandler *NhnCloudSecurityHandler) RemoveRules(sgIID irs.IID, securityRules *[]irs.SecurityRuleInfo) (bool, error) {
	cblogger.Info("NHN Cloud Driver: called RemoveRules()!")	
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, sgIID.SystemId, "RemoveRules()")

	if sgIID.SystemId == "" {
		newErr := fmt.Errorf("Invalid S/G SystemId!!")
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return false, newErr
	}

	// Check if the S/G exists
	sgInfo, err := securityHandler.GetSecurity(sgIID)
	if err != nil {
		newErr := fmt.Errorf("Failed to Find any S/G info. with the SystemId : [%s] : [%v]", sgIID.SystemId, err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return false, newErr
	}

	cblogger.Infof("S/G SystemId to Remove the Rules [%s]", sgInfo.IId.SystemId)

	// Deletge the given S/G Rules
	for _, curRule := range *securityRules {
		if curRule.Direction == "" {
			return false, errors.New("Failed to Find 'Direction' Value in the requested rule!!")
		} else if curRule.IPProtocol == "" {
			return false, errors.New("Failed to Find 'IPProtocol' Value in the requested rule!!")
		} else if curRule.FromPort == "" {
			return false, errors.New("Failed to Find 'FromPort' Value in the requested rule!!")
		} else if curRule.ToPort == "" {
			return false, errors.New("Failed to Find 'ToPort' Value in the requested rule!!")
		} else if curRule.CIDR == "" {
			return false, errors.New("Failed to Find 'CIDR' Value in the requested rule!!")
		}

		cblogger.Infof("curRule.IPProtocol : [%s]", curRule.IPProtocol)
		
		if strings.EqualFold(curRule.IPProtocol, "ALL") { 	// Add SecurityGroup Rules in case of 'All Traffic Open Rule'
			if strings.EqualFold(curRule.FromPort, "-1") && strings.EqualFold(curRule.ToPort, "-1") {
				var direction string
				if strings.EqualFold(curRule.Direction, "inbound") {
					direction = "inbound"
				} else if strings.EqualFold(curRule.Direction, "outbound") {
					direction = "outbound"
				} else {
					return false, errors.New("Invalid Rule Direction!!")
				}

				allProtocolTypeCode := []string {"tcp", "udp", "icmp"}
				allCIDR := "0.0.0.0/0"

				for _, curProtocolType := range allProtocolTypeCode {
					var ruleInfo irs.SecurityRuleInfo	
					if strings.EqualFold(curProtocolType, "icmp") {
						ruleInfo = irs.SecurityRuleInfo {
							Direction: direction,
							IPProtocol: curProtocolType,
							FromPort: "-1",
							ToPort: "-1",
							CIDR: allCIDR,
						}
					} else {
						ruleInfo = irs.SecurityRuleInfo {
							Direction: direction,
							IPProtocol: curProtocolType,
							FromPort: "1",
							ToPort: "65535",
							CIDR: allCIDR,
						}
					}
		
					// Get the Rule ID from the S/G
					ruleId, err := securityHandler.GetRuleIdFromRuleInfo(sgIID, ruleInfo)
					if err != nil {
						newErr := fmt.Errorf("Failed to Find any S/G info. with the SystemId : [%s] : [%v]", sgIID.SystemId, err)
						cblogger.Error(newErr.Error())
						LoggingError(callLogInfo, newErr)
						return false, newErr
					}
				
					cblogger.Infof("The RuleID of Current Rule : ", ruleId)

					// Delete the Rule
					start := call.Start()
					delResult := rules.Delete(securityHandler.NetworkClient, ruleId)
					LoggingInfo(callLogInfo, start)
					if delResult.Err != nil {
						newErr := fmt.Errorf("Failed to Remove Rules of the S/G : [%s] : [%v]", sgIID.SystemId, delResult.Err)
						cblogger.Error(newErr.Error())
						LoggingError(callLogInfo, newErr)
						return false, newErr
					}
					LoggingInfo(callLogInfo, start)
					// spew.Dump(delResult)

					cblogger.Infof("Succeeded in Removing the [%s], [%s] Rule!!", direction, curProtocolType)
				}
			} else {
				return false, errors.New("To Specify 'All Traffic Allow Rule', Specify '-1' as FromPort/ToPort!!")
			}
		} else {
			// Get the Rule ID from the S/G
			ruleId, err := securityHandler.GetRuleIdFromRuleInfo(sgIID, curRule)
			if err != nil {
				newErr := fmt.Errorf("Failed to Find any S/G info. with the SystemId : [%s], [%v]", sgIID.SystemId, err)
				cblogger.Error(newErr.Error())
				LoggingError(callLogInfo, newErr)
				return false, newErr
			}
		
			cblogger.Infof("The RuleID of Current Rule : ", ruleId)

			// Delete the Rule
			start := call.Start()
			delResult := rules.Delete(securityHandler.NetworkClient, ruleId)
			if delResult.Err != nil {
				newErr := fmt.Errorf("Failed to Remove Rules of the S/G : [%s] : [%v]", sgIID.SystemId, delResult.Err)
				cblogger.Error(newErr.Error())
				LoggingError(callLogInfo, newErr)
				return false, newErr
			}
			LoggingInfo(callLogInfo, start)
			// spew.Dump(delResult)

			cblogger.Infof("Succeeded in Removing the [%s], [%s] Rule!!", curRule.Direction, curRule.IPProtocol)	
		}
	}

	// // AddServer will associate a server and a security group, enforcing the rules of the group on the server.		
	// addServerResult := secgroups.AddServer(securityHandler.VMClient, serverID, securityIID.NameId)

	// // RemoveServer will disassociate a server from a security group
	// removeServerResult := secgroups.RemoveServer(securityHandler.VMClient, serverID, securityIID.NameId)

	return true, nil
}

func (securityHandler *NhnCloudSecurityHandler) OpenOutboundAllProtocol(sgIID irs.IID) (error) {
	cblogger.Info("NHN Cloud driver: called OpenOutboundAllProtocol()!")	
    callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, sgIID.SystemId, "OpenOutboundAllProtocol()")

	reqRules := []irs.SecurityRuleInfo {
			{
				Direction: 		"outbound",
				IPProtocol:  	"ALL",
				FromPort: 		"-1",
				ToPort: 		"-1",
				CIDR: 			"0.0.0.0/0",		
			},
		}
	
	_, err := securityHandler.AddRules(sgIID, &reqRules)
	if err != nil {
		newErr := fmt.Errorf("Failed to Add Outbound All Protocol Opening Rule. : [%v]", err)
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return newErr
	}

	return nil
}

func (securityHandler *NhnCloudSecurityHandler) MappingSecurityInfo(nhnSG secgroups.SecurityGroup, defaultVPCSystemId string) (*irs.SecurityInfo, error) {
	cblogger.Info("NHN Cloud Driver: called MappingSecurityInfo()!")
	// spew.Dump(nhnSG)

	secInfo := &irs.SecurityInfo{
		IId: irs.IID{
			NameId:   nhnSG.Name,
			SystemId: nhnSG.ID,
		},

		VpcIID: irs.IID {
			NameId:   "",
			SystemId: defaultVPCSystemId,
		},

		KeyValueList: []irs.KeyValue{
			{Key: "TenantID", Value: nhnSG.TenantID},
		},
	}

	listOpts := rules.ListOpts{
		SecGroupID: nhnSG.ID,
	}

	allPages, err := rules.List(securityHandler.NetworkClient, listOpts).AllPages()
	if err != nil {
		cblogger.Error(err.Error())
		return nil, err
	}

	nhnRuleList, err := rules.ExtractRules(allPages)
	if err != nil {
		cblogger.Error(err.Error())
		return nil, err
	}

	if len(nhnRuleList) < 1 {
		cblogger.Infof("$$$ The S/G [%s] contains No Rule!!", nhnSG.ID)
		// return nil, nil // Caution!!
	} else {
		// Set Security Rule info. list
		var sgRuleList []irs.SecurityRuleInfo
		for _, nhnRule := range nhnRuleList {
			if !strings.EqualFold(nhnRule.Protocol, "") {   // Since on NHN Cloud Console ... 
				var direction string
				if strings.EqualFold(nhnRule.Direction, string(rules.DirIngress)) {
					direction = "inbound"
				} else if strings.EqualFold(nhnRule.Direction, string(rules.DirEgress)) {
					direction = "outbound"
				} else {
					return nil, errors.New("Invalid Rule Direction!!")
				}

				ruleInfo := irs.SecurityRuleInfo{
					Direction:  direction,
					IPProtocol: strings.ToLower(nhnRule.Protocol),
					CIDR:       nhnRule.RemoteIPPrefix,
				}

				if strings.EqualFold(nhnRule.Protocol, "icmp") {
					ruleInfo.FromPort = "-1"
					ruleInfo.ToPort = "-1"
				} else {
					ruleInfo.FromPort = strconv.Itoa(nhnRule.PortRangeMin)
					ruleInfo.ToPort = strconv.Itoa(nhnRule.PortRangeMax)
				}

				sgRuleList = append(sgRuleList, ruleInfo)
			}
		}

		secInfo.SecurityRules = &sgRuleList
	}
	return secInfo, nil
}

func (securityHandler *NhnCloudSecurityHandler) GetRuleIdFromRuleInfo(sgIID irs.IID, givenRule irs.SecurityRuleInfo) (string, error) {
	cblogger.Info("NHN Cloud Driver: called GetRuleIdFromRuleInfo()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.SECURITYGROUP, sgIID.SystemId, "GetRuleIdFromRuleInfo()")

	// Get NHN Cloud S/G Raw Info
	nhnSG, err := secgroups.Get(securityHandler.VMClient, sgIID.SystemId).Extract()
	if err != nil {
		cblogger.Error(err.Error())
		LoggingError(callLogInfo, err)
		return "", err
	}

	listOpts := rules.ListOpts{
		SecGroupID: nhnSG.ID,
	}

	allPages, err := rules.List(securityHandler.NetworkClient, listOpts).AllPages()
	if err != nil {
		cblogger.Error(err.Error())
		return "", err
	}

	nhnRuleList, err := rules.ExtractRules(allPages)
	if err != nil {
		cblogger.Error(err.Error())
		return "", err
	}
	// spew.Dump(nhnRuleList)

	var ruleId string

	if len(nhnRuleList) < 1 {
		cblogger.Infof("$$$ The S/G [%s] contains No Rule!!", nhnSG.ID)
		return "", nil // Caution!!
	} else {
		// Set Security Rule info. list
		for _, nhnRule := range nhnRuleList {
			if !strings.EqualFold(nhnRule.Protocol, "") {   // Since on NHN Cloud Console ... 
				var direction string
				if strings.EqualFold(nhnRule.Direction, string(rules.DirIngress)) {
					direction = "inbound"
				} else if strings.EqualFold(nhnRule.Direction, string(rules.DirEgress)) {
					direction = "outbound"
				} else {
					return "", errors.New("Invalid Rule Direction!!")
				}

				var fromPort string
				var toPort string
				if strings.EqualFold(nhnRule.Protocol, "icmp") {
					fromPort = "-1"  // Caution : Not strconv.Itoa(0) 
					toPort = "-1"  	// Caution : Not strconv.Itoa(0)
				} else {
					fromPort = strconv.Itoa(nhnRule.PortRangeMin)
					toPort = strconv.Itoa(nhnRule.PortRangeMax)
				}				
				
				if strings.EqualFold(givenRule.Direction, direction) && strings.EqualFold(givenRule.IPProtocol, nhnRule.Protocol) && strings.EqualFold(givenRule.FromPort, fromPort) && strings.EqualFold(givenRule.ToPort, toPort) && strings.EqualFold(givenRule.CIDR, nhnRule.RemoteIPPrefix) {
					ruleId = nhnRule.ID
					break
				}				
			}
		}
	}

	if strings.EqualFold(ruleId, "") {
		return "", errors.New("Failed to Find RuleID with the Given S/G Rule!!")
	}
	return ruleId, nil
}

func (securityHandler *NhnCloudSecurityHandler) GetDefaultVPCSystemID() (string, error) {
	cblogger.Info("NHN Cloud Cloud Driver: called GetDefaultVPCSystemID()!")
	callLogInfo := GetCallLogScheme(securityHandler.RegionInfo.Region, call.VPCSUBNET, "GetDefaultVPCSystemID()", "GetDefaultVPCSystemID()")

	var vpcSystemId string

	listOpts := external.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{},
	}

	start := call.Start()
	allPages, err := networks.List(securityHandler.NetworkClient, listOpts).AllPages()
	if err != nil {
		cblogger.Errorf("Failed to Get Network list from NHN Cloud. : [%v]", err)
		LoggingError(callLogInfo, err)
		return "", err
	}
	LoggingInfo(callLogInfo, start)

	// To Get VPC info list
	var vpcList []NetworkWithExt
	err = networks.ExtractNetworksInto(allPages, &vpcList)
	if err != nil {
		cblogger.Errorf("Failed to Get VPC list from NHN Cloud. : [%v]", err)
		LoggingError(callLogInfo, err)
		return "", err
	}

	for _, vpc := range vpcList {
		if strings.EqualFold(vpc.Name, "Default Network") {
			vpcSystemId = vpc.ID
			cblogger.Infof("# SystemId of the Default VPC : [%s]", vpcSystemId)
			break
		}
	}

	// When the "Default Network" VPC is not found
	if strings.EqualFold(vpcSystemId, "") {
		newErr := fmt.Errorf("Failed to Find the 'Default Network' VPC on your NHN Cloud project!!")
		cblogger.Error(newErr.Error())
		LoggingError(callLogInfo, newErr)
		return "", newErr		
	}
	return vpcSystemId, nil
}
