package main

import (
	"fmt"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

type ResourcePolicy struct {
	MaxCPU    string
	MaxMemory string
}

func validateManifest(manifest string, policy ResourcePolicy) error {
	docs := splitYAMLDocs(manifest)
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return fmt.Errorf("invalid manifest yaml: %w", err)
		}
		if err := checkSecurity(obj); err != nil {
			return err
		}
		if err := checkResourceLimits(obj, policy); err != nil {
			return err
		}
	}
	return nil
}

func splitYAMLDocs(input string) []string {
	return strings.Split(input, "\n---")
}

func checkSecurity(obj map[string]any) error {
	spec := nestedMap(obj, "spec", "template", "spec")
	if spec == nil {
		spec = nestedMap(obj, "spec")
	}
	if spec == nil {
		return nil
	}

	if vols, ok := spec["volumes"].([]any); ok {
		for _, v := range vols {
			vm, _ := v.(map[string]any)
			if vm == nil {
				continue
			}
			if _, has := vm["hostPath"]; has {
				return fmt.Errorf("hostPath volume is not allowed")
			}
		}
	}

	containers, _ := spec["containers"].([]any)
	for _, c := range containers {
		cm, _ := c.(map[string]any)
		if cm == nil {
			continue
		}
		if sc, ok := cm["securityContext"].(map[string]any); ok {
			if privileged, ok := sc["privileged"].(bool); ok && privileged {
				return fmt.Errorf("privileged container is not allowed")
			}
		}
	}
	return nil
}

func checkResourceLimits(obj map[string]any, policy ResourcePolicy) error {
	maxCPU, err := parseCPU(policy.MaxCPU)
	if err != nil {
		return fmt.Errorf("invalid cpu policy: %w", err)
	}
	maxMem, err := parseMemoryMi(policy.MaxMemory)
	if err != nil {
		return fmt.Errorf("invalid memory policy: %w", err)
	}

	spec := nestedMap(obj, "spec", "template", "spec")
	if spec == nil {
		spec = nestedMap(obj, "spec")
	}
	if spec == nil {
		return nil
	}

	containers, _ := spec["containers"].([]any)
	for _, c := range containers {
		cm, _ := c.(map[string]any)
		if cm == nil {
			continue
		}
		res, _ := cm["resources"].(map[string]any)
		limits, _ := res["limits"].(map[string]any)
		if limits == nil {
			return fmt.Errorf("resources.limits is required for each container")
		}
		cpuVal, ok := limits["cpu"]
		if !ok {
			return fmt.Errorf("cpu limit is required")
		}
		memVal, ok := limits["memory"]
		if !ok {
			return fmt.Errorf("memory limit is required")
		}

		cpu, err := parseCPU(fmt.Sprintf("%v", cpuVal))
		if err != nil {
			return fmt.Errorf("invalid cpu limit: %w", err)
		}
		mem, err := parseMemoryMi(fmt.Sprintf("%v", memVal))
		if err != nil {
			return fmt.Errorf("invalid memory limit: %w", err)
		}
		if cpu > maxCPU {
			return fmt.Errorf("cpu limit %v exceeds policy %s", cpuVal, policy.MaxCPU)
		}
		if mem > maxMem {
			return fmt.Errorf("memory limit %v exceeds policy %s", memVal, policy.MaxMemory)
		}
	}
	return nil
}

func nestedMap(m map[string]any, keys ...string) map[string]any {
	curr := m
	for _, k := range keys {
		v, ok := curr[k]
		if !ok {
			return nil
		}
		next, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		curr = next
	}
	return curr
}

func parseCPU(cpu string) (int, error) {
	cpu = strings.TrimSpace(cpu)
	if strings.HasSuffix(cpu, "m") {
		v := strings.TrimSuffix(cpu, "m")
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
	f, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, err
	}
	return int(f * 1000), nil
}

func parseMemoryMi(mem string) (int, error) {
	mem = strings.TrimSpace(mem)
	switch {
	case strings.HasSuffix(mem, "Gi"):
		v := strings.TrimSuffix(mem, "Gi")
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return i * 1024, nil
	case strings.HasSuffix(mem, "Mi"):
		v := strings.TrimSuffix(mem, "Mi")
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		i, err := strconv.Atoi(mem)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
}
