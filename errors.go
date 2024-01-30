package main

import "errors"

var errConfigurationCheckFailed = errors.New("unable to check configuration of container")
var errGatewayPathUnset = errors.New("container has no gateway path configured")
var errGatewayPathEmpty = errors.New("container has empty gateway path configured")
var errPortParseFail = errors.New("container has invalid port configured")
var errAuthParseFail = errors.New("container has invalid authentication directive configured")
