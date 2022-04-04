#!/bin/bash

if [ "$1" = "" ]; then
	echo
	echo -e 'usage: '$0' aws|gcp|alibaba|azure|openstack|cloudit|tencent|nhncloud'
	echo -e '\n\tex) '$0' aws'
	echo
	exit 0;
fi

source ./setup.env $1

echo "============== before create VPC/Subnet: '${VPC_NAME}'"
time $CLIPATH/spctl --config $CLIPATH/spctl.conf vpc create -i json -d \
    '{
      "ConnectionName":"'${CONN_CONFIG}'",
      "ReqInfo": {
        "Name": "'${VPC_NAME}'",
        "IPv4_CIDR": "'${VPC_CIDR}'",
        "SubnetInfoList": [
          {
            "Name": "'${SUBNET_NAME}'",
            "IPv4_CIDR": "'${SUBNET_CIDR}'"
          }
        ]
      }
    }' 2> /dev/null

echo "============== after create VPC/Subnet: '${VPC_NAME}'"


echo "============== before create SecurityGroup: '${SG_NAME}'"
time $CLIPATH/spctl --config $CLIPATH/spctl.conf security create -i json -d \
    '{
      "ConnectionName":"'${CONN_CONFIG}'",
      "ReqInfo": {
        "Name": "'${SG_NAME}'",
        "VPCName": "'${VPC_NAME}'",
        "SecurityRules": [
          {
            "FromPort": "1",
            "ToPort" : "65535",
            "IPProtocol" : "tcp",
            "Direction" : "inbound"
          }
        ]
      }
    }' 2> /dev/null
echo "============== after create SecurityGroup: '${SG_NAME}'"


echo "============== before create KeyPair: '${KEYPAIR_NAME}'"
time $CLIPATH/spctl --config $CLIPATH/spctl.conf keypair create -i json -d \
    '{
      "ConnectionName":"'${CONN_CONFIG}'",
      "ReqInfo": {
        "Name": "'${KEYPAIR_NAME}'"
      }
    }' 2> /dev/null
echo "============== after create KeyPair: '${KEYPAIR_NAME}'"

