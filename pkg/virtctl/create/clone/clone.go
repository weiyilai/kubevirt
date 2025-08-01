/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package clone

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	clone "kubevirt.io/api/clone/v1beta1"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/yaml"

	"kubevirt.io/kubevirt/pkg/pointer"
	"kubevirt.io/kubevirt/pkg/virtctl/clientconfig"
)

const (
	Clone = "clone"

	NameFlag                     = "name"
	SourceNameFlag               = "source-name"
	TargetNameFlag               = "target-name"
	SourceTypeFlag               = "source-type"
	TargetTypeFlag               = "target-type"
	LabelFilterFlag              = "label-filter"
	AnnotationFilterFlag         = "annotation-filter"
	TemplateLabelFilterFlag      = "template-label-filter"
	TemplateAnnotationFilterFlag = "template-annotation-filter"
	NewMacAddressesFlag          = "new-mac-address"
	NewSMBiosSerialFlag          = "new-smbios-serial"

	supportedSourceTypes = "vm, vmsnapshot"
	supportedTargetTypes = "vm"
)

type createClone struct {
	namespace                 string
	name                      string
	sourceName                string
	targetName                string
	sourceType                string
	targetType                string
	labelFilters              []string
	annotationFilters         []string
	templateLabelFilters      []string
	templateAnnotationFilters []string
	newMacAddresses           []string
	newSmbiosSerial           string
}

type (
	cloneSpec clone.VirtualMachineCloneSpec
	optionFn  func(*createClone, *cloneSpec) error
)

var optFns = map[string]optionFn{
	NewMacAddressesFlag: withNewMacAddresses,
}

func NewCommand() *cobra.Command {
	c := createClone{}
	cmd := &cobra.Command{
		Use:     Clone,
		Short:   "Create a clone object manifest",
		Example: c.usage(),
		RunE:    c.run,
	}

	const emptyValue = ""
	const supportsMultipleFlags = "Can be provided multiple times."

	cmd.Flags().StringVar(&c.name, NameFlag, emptyValue,
		"Specify the name of the clone. If not specified, name would be randomized.")
	cmd.Flags().StringVar(&c.sourceName, SourceNameFlag, emptyValue,
		"Specify the clone's source name.")
	cmd.Flags().StringVar(&c.targetName, TargetNameFlag, emptyValue,
		"Specify the clone's target name.")
	cmd.Flags().StringVar(&c.sourceType, SourceTypeFlag, emptyValue,
		"Specify the clone's source type. Default type is VM. Supported types: "+supportedSourceTypes)
	cmd.Flags().StringVar(&c.targetType, TargetTypeFlag, emptyValue,
		"Specify the clone's target type. Default type is VM. Supported types: "+supportedTargetTypes)
	cmd.Flags().StringArrayVar(&c.labelFilters, LabelFilterFlag, nil,
		"Specify clone's label filters. "+supportsMultipleFlags)
	cmd.Flags().StringArrayVar(&c.annotationFilters, AnnotationFilterFlag, nil,
		"Specify clone's annotation filters. "+supportsMultipleFlags)
	cmd.Flags().StringArrayVar(&c.templateLabelFilters, TemplateLabelFilterFlag, nil,
		"Specify clone's template label filters. "+supportsMultipleFlags)
	cmd.Flags().StringArrayVar(&c.templateAnnotationFilters, TemplateAnnotationFilterFlag, nil,
		"Specify clone's template annotation filters. "+supportsMultipleFlags)
	cmd.Flags().StringArrayVar(&c.newMacAddresses, NewMacAddressesFlag, nil,
		"Specify clone's new mac addresses. For example: 'interfaceName0:newAddress0'")
	cmd.Flags().StringVar(&c.newSmbiosSerial, NewSMBiosSerialFlag, emptyValue,
		"Specify the clone's new smbios serial")

	if err := cmd.MarkFlagRequired(SourceNameFlag); err != nil {
		panic(err)
	}

	return cmd
}

func withNewMacAddresses(c *createClone, cloneSpec *cloneSpec) error {
	const expectedMacParts = 2
	for _, param := range c.newMacAddresses {
		splitParam := strings.Split(param, ":")
		if len(splitParam) != expectedMacParts {
			return fmt.Errorf("newMacAddress parameter %s is invalid: exactly one ':' is expected. For example: 'interface0:address0'", param)
		}

		interfaceName, newAddress := splitParam[0], splitParam[1]
		if interfaceName == "" {
			return fmt.Errorf("newMacAddress parameter %s has empty interface name", param)
		}
		if newAddress == "" {
			return fmt.Errorf("newMacAddress parameter %s has empty address name", param)
		}

		if cloneSpec.NewMacAddresses == nil {
			cloneSpec.NewMacAddresses = map[string]string{}
		}

		cloneSpec.NewMacAddresses[interfaceName] = newAddress
	}

	return nil
}

func (c *createClone) usage() string {
	return `  # Create a manifest for a clone with a random name:
  {{ProgramName}} create clone --source-name sourceVM --target-name targetVM

  # Create a manifest for a clone with a specified name:
  {{ProgramName}} create clone --name my-clone --source-name sourceVM --target-name targetVM

  # Create a manifest for a clone with a randomized target name (target name is omitted):
  {{ProgramName}} create clone --source-name sourceVM

  # Create a manifest for a clone with specified source / target types. The default type is VM.
  {{ProgramName}} create clone --source-name sourceVM --source-type vm --target-name targetVM --target-type vm

  # Supported source types are vm (aliases: VM, VirtualMachine, virtualmachine) and snapshot (aliases: vmsnapshot
  # VirtualMachineSnapshot, VMSnapshot). The only supported target type is vm.

  # Create a manifest for a clone with a source type snapshot to a target type VM:
  {{ProgramName}} create clone --source-name mySnapshot --source-type snapshot --target-name targetVM

  # Create a manifest for a clone with label filters:
  {{ProgramName}} create clone --source-name sourceVM --label-filter '*' --label-filter '!some/key'

  # Create a manifest for a clone with annotation filters:
  {{ProgramName}} create clone --source-name sourceVM --annotation-filter '*' --annotation-filter '!some/key'

  # Create a manifest for a clone with template label filters:
  {{ProgramName}} create clone --source-name sourceVM --template-label-filter '*' --template-label-filter '!some/key'

  # Create a manifest for a clone with template annotation filters:
  {{ProgramName}} create clone --source-name sourceVM --template-annotation-filter '*' --template-annotation-filter '!some/key'

  # Create a manifest for a clone with new MAC addresses:
  {{ProgramName}} create clone --source-name sourceVM --new-mac-address interface1:00-11-22 --new-mac-address interface2:00-11-33

  # Create a manifest for a clone with new SMBIOS serial:
  {{ProgramName}} create clone --source-name sourceVM --new-smbios-serial "new-serial"

  # Create a manifest for a clone and use it to create a resource with kubectl
  {{ProgramName}} create clone --source-name sourceVM | kubectl create -f -`
}

func (c *createClone) newClone() (*clone.VirtualMachineClone, error) {
	vmClone := kubecli.NewMinimalClone(c.name)
	if c.namespace != "" {
		vmClone.Namespace = c.namespace
	}

	source, err := c.typeToTypedLocalObjectReference(c.sourceType, c.sourceName, true)
	if err != nil {
		return nil, err
	}

	target, err := c.typeToTypedLocalObjectReference(c.targetType, c.targetName, false)
	if err != nil {
		return nil, err
	}

	vmClone.Spec = clone.VirtualMachineCloneSpec{
		Source:            source,
		Target:            target,
		AnnotationFilters: c.annotationFilters,
		LabelFilters:      c.labelFilters,
		Template: clone.VirtualMachineCloneTemplateFilters{
			AnnotationFilters: c.templateAnnotationFilters,
			LabelFilters:      c.templateLabelFilters,
		},
	}

	if c.newSmbiosSerial != "" {
		vmClone.Spec.NewSMBiosSerial = pointer.P(c.newSmbiosSerial)
	}

	return vmClone, nil
}

func (c *createClone) applyFlags(cmd *cobra.Command, spec *clone.VirtualMachineCloneSpec) error {
	for flag := range optFns {
		if cmd.Flags().Changed(flag) {
			if err := optFns[flag](c, (*cloneSpec)(spec)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *createClone) run(cmd *cobra.Command, _ []string) error {
	const cloneRandomSuffixLength = 5
	if err := c.setDefaults(cmd.Context()); err != nil {
		return err
	}

	err := c.validateFlags()
	if err != nil {
		return err
	}

	vmClone, err := c.newClone()
	if err != nil {
		return err
	}

	err = c.applyFlags(cmd, &vmClone.Spec)
	if err != nil {
		return err
	}

	if vmClone.Name == "" {
		vmClone.Name = "clone-" + rand.String(cloneRandomSuffixLength)
	}

	cloneBytes, err := yaml.Marshal(vmClone)
	if err != nil {
		return err
	}

	cmd.Print(string(cloneBytes))
	return nil
}

func (c *createClone) typeToTypedLocalObjectReference(sourceOrTargetType, sourceOrTargetName string,
	isSource bool,
) (*v1.TypedLocalObjectReference, error) {
	const (
		virtualMachineKind         = "VirtualMachine"
		virtualMachineSnapshotKind = "VirtualMachineSnapshot"
	)
	var kind, apiGroup string

	generateErr := func() error {
		var sourceTargetStr string
		var supportedTypes string

		if isSource {
			sourceTargetStr = "source"
			supportedTypes = supportedSourceTypes
		} else {
			sourceTargetStr = "target"
			supportedTypes = supportedTargetTypes
		}

		return fmt.Errorf("%s type %s is not supported. Supported types: %s", sourceTargetStr, sourceOrTargetType, supportedTypes)
	}

	switch sourceOrTargetType {
	case "vm", "VM", virtualMachineKind, "virtualmachine":
		kind = virtualMachineKind
		apiGroup = "kubevirt.io"
	case "snapshot", virtualMachineSnapshotKind, "vmsnapshot", "VMSnapshot":
		if !isSource {
			return nil, generateErr()
		}

		kind = virtualMachineSnapshotKind
		apiGroup = "snapshot.kubevirt.io"
	default:
		return nil, generateErr()
	}

	return &v1.TypedLocalObjectReference{
		Name:     sourceOrTargetName,
		Kind:     kind,
		APIGroup: &apiGroup,
	}, nil
}

func (c *createClone) setDefaults(ctx context.Context) error {
	_, namespace, overridden, err := clientconfig.ClientAndNamespaceFromContext(ctx)
	const cloneRandomSuffixLength = 5
	if err != nil {
		return err
	}
	if overridden {
		c.namespace = namespace
	}

	if c.name == "" {
		c.name = "clone-" + rand.String(cloneRandomSuffixLength)
	}

	const defaultType = "vm"

	if c.sourceType == "" {
		c.sourceType = defaultType
	}
	if c.targetType == "" {
		c.targetType = defaultType
	}

	return nil
}

func (c *createClone) validateFlags() error {
	if c.sourceName == "" {
		return fmt.Errorf("source name must not be empty")
	}

	return nil
}
