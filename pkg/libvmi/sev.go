/*
Copyright The KubeVirt Authors.

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

package libvmi

import v1 "kubevirt.io/api/core/v1"

// WithSEV adds `launchSecurity` with `sev`.
func WithSEV(isESEnabled bool) Option {
	return func(vmi *v1.VirtualMachineInstance) {
		vmi.Spec.Domain.LaunchSecurity = &v1.LaunchSecurity{
			SEV: &v1.SEV{
				Policy: &v1.SEVPolicy{
					EncryptedState: &isESEnabled,
				},
			},
		}
	}
}

func WithSEVAttestation() Option {
	return func(vmi *v1.VirtualMachineInstance) {
		startStrategy := v1.StartStrategyPaused
		vmi.Spec.StartStrategy = &startStrategy
		if vmi.Spec.Domain.LaunchSecurity == nil {
			vmi.Spec.Domain.LaunchSecurity = &v1.LaunchSecurity{}
		}
		if vmi.Spec.Domain.LaunchSecurity.SEV == nil {
			vmi.Spec.Domain.LaunchSecurity.SEV = &v1.SEV{}
		}
		vmi.Spec.Domain.LaunchSecurity.SEV.Attestation = &v1.SEVAttestation{}
	}
}
