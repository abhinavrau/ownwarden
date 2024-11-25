package test

import (
	"net"

	"context"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"net/http"
	"tailscale.com/tsnet"
	"testing"
)

func TestOwnWarndenEnd2End(t *testing.T) {

	// retryable errors in terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// Set the path to the Terraform code that will be tested.
		TerraformDir: "../",
		// Variables to pass to our Terraform code using -var-file options
		//VarFiles: []string{"end2end_test.tfvars"},
		VarFiles: []string{"terraform.tfvars"},
		// Disable colors in Terraform commands so its easier to parse stdout/stderr
		NoColor: true,
	})

	tailscale_auth_key := terraform.GetVariableAsStringFromVarFile(t, "../terraform.tfvars", "tailscale_auth_key")
	tailscale_hostname := terraform.GetVariableAsStringFromVarFile(t, "../terraform.tfvars", "tailscale_hostname")
	tailscale_domain := terraform.GetVariableAsStringFromVarFile(t, "../terraform.tfvars", "tailscale_domain")

	// Clean up resources with "terraform destroy" at the end of the test.

	defer test_structure.RunTestStage(t, "teardown", func() {

		terraform.Destroy(t, terraformOptions)

	})

	// Deploy the example
	test_structure.RunTestStage(t, "setup", func() {
		//  Run "terraform init" and "terraform apply". Fail the test if there are any errors.
		terraform.InitAndApply(t, terraformOptions)
	})

	test_structure.RunTestStage(t, "validate", func() {

		// validate IP address
		output_external_ip := terraform.Output(t, terraformOptions, "external_ip")
		assert.NotNil(t, net.ParseIP(output_external_ip))

		checkOwnWardenService(t, tailscale_auth_key, tailscale_hostname, tailscale_domain)
	})

}

func checkOwnWardenService(t *testing.T, tailscale_auth_key, tailscale_hostname, tailscale_domain string) {
	srv := new(tsnet.Server)
	srv.Hostname = "terratest"
	srv.Ephemeral = true
	srv.AuthKey = tailscale_auth_key

	ctx := context.Background()
	status, err := srv.Up(ctx)
	if err != nil {
		assert.Fail(t, "Tailscale Service failed to start.", "err = %v", err)
	}
	defer srv.Close()

	assert.True(t, status.TailscaleIPs[0].IsValid())

	conn := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return srv.Dial(ctx, "tcp", tailscale_hostname+":443")
			},
		},
	}

	resp, err := conn.Get("https://" + tailscale_domain)
	if err != nil {
		assert.Fail(t, "HTTPS call failed", "err = %v", err)
	}

	assert.Equal(t, 200, resp.StatusCode, "Call to ownwarden web interface failed")
	assert.NotZero(t, resp.ContentLength, "Body is empty")

}
