package config

import "testing"

func TestDockerAgentTokenDirRules(t *testing.T) {
	c := liveConfig()
	c.DockerAgent.TokenDir = "/run/docker-agent-tokens"
	if err := c.Validate(); err != nil {
		t.Fatalf("live config with a docker token directory: %v", err)
	}
	c.HostAgent.TokenDir = "/run/docker-agent-tokens"
	if err := c.Validate(); err == nil {
		t.Fatal("the docker and host agent token directories must differ")
	}
	demo := liveConfig()
	demo.Features.DemoProvider = true
	demo.DockerAgent.TokenDir = "/run/docker-agent-tokens"
	if err := demo.Validate(); err == nil {
		t.Fatal("the docker reporter endpoint must need live inventory")
	}
}
