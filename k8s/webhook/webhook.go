/*
Copyright 2020 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	webhookcontroller "github.com/sky-big/go-common/k8s/webhook/controller"
	"github.com/sky-big/go-common/k8s/webhook/health"
	webhookutil "github.com/sky-big/go-common/k8s/webhook/util"
)

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;update;patch

func Run(ctx context.Context, mgr manager.Manager, handlerMap map[string]admission.Handler) error {
	if err := mgr.AddReadyzCheck("webhook-ready", health.Checker); err != nil {
		return fmt.Errorf("unable to add readyz check")
	}

	c, err := webhookcontroller.New(mgr.GetConfig(), handlerMap)
	if err != nil {
		return err
	}
	go func() {
		c.Start(ctx)
	}()

	timer := time.NewTimer(time.Second * 20)
	defer timer.Stop()
	select {
	case <-webhookcontroller.Inited():
		return waitReady()
	case <-timer.C:
		return fmt.Errorf("failed to start webhook controller for waiting more than 5s")
	}

	server := mgr.GetWebhookServer()
	server.Host = "0.0.0.0"
	server.Port = webhookutil.GetPort()
	server.CertDir = webhookutil.GetCertDir()
	return nil
}

func waitReady() error {
	startTS := time.Now()
	var err error
	for {
		duration := time.Since(startTS)
		if err = health.Checker(nil); err == nil {
			return nil
		}

		if duration > time.Second*5 {
			klog.Warningf("Failed to wait webhook ready over %s: %v", duration, err)
		}
		if duration > time.Minute {
			return err
		}
		time.Sleep(time.Second * 2)
	}
}
