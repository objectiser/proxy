// Copyright 2019 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"testing"
	"text/template"

	"istio.io/proxy/test/envoye2e/driver"
	"istio.io/proxy/test/envoye2e/env"
)

const metadataExchangeIstioConfigFilter = `
- name: envoy.filters.network.metadata_exchange
  config:
    protocol: istio2
- name: envoy.filters.network.wasm
  config:
    config:
      root_id: "stats_inbound"
      vm_config:
        runtime: envoy.wasm.runtime.null
        code:
          local: { inline_string: "envoy.wasm.stats" }
      configuration: |
        { "debug": "false", max_peer_cache_size: 20, field_separator: ";.;" }
`

const statsConfig = `stats_config:
  use_all_default_tags: true
  stats_tags:
  - tag_name: "reporter"
    regex: "(reporter=\\.=(.+?);\\.;)"
  - tag_name: "source_namespace"
    regex: "(source_namespace=\\.=(.+?);\\.;)"
  - tag_name: "source_workload"
    regex: "(source_workload=\\.=(.+?);\\.;)"
  - tag_name: "source_workload_namespace"
    regex: "(source_workload_namespace=\\.=(.+?);\\.;)"
  - tag_name: "source_principal"
    regex: "(source_principal=\\.=(.+?);\\.;)"
  - tag_name: "source_app"
    regex: "(source_app=\\.=(.+?);\\.;)"
  - tag_name: "source_version"
    regex: "(source_version=\\.=(.+?);\\.;)"
  - tag_name: "destination_namespace"
    regex: "(destination_namespace=\\.=(.+?);\\.;)"
  - tag_name: "destination_workload"
    regex: "(destination_workload=\\.=(.+?);\\.;)"
  - tag_name: "destination_workload_namespace"
    regex: "(destination_workload_namespace=\\.=(.+?);\\.;)"
  - tag_name: "destination_principal"
    regex: "(destination_principal=\\.=(.+?);\\.;)"
  - tag_name: "destination_app"
    regex: "(destination_app=\\.=(.+?);\\.;)"
  - tag_name: "destination_version"
    regex: "(destination_version=\\.=(.+?);\\.;)"
  - tag_name: "destination_service"
    regex: "(destination_service=\\.=(.+?);\\.;)"
  - tag_name: "destination_service_name"
    regex: "(destination_service_name=\\.=(.+?);\\.;)"
  - tag_name: "destination_service_namespace"
    regex: "(destination_service_namespace=\\.=(.+?);\\.;)"
  - tag_name: "destination_port"
    regex: "(destination_port=\\.=(.+?);\\.;)"
  - tag_name: "request_protocol"
    regex: "(request_protocol=\\.=(.+?);\\.;)"
  - tag_name: "response_code"
    regex: "(response_code=\\.=(.+?);\\.;)|_rq(_(\\.d{3}))$"
  - tag_name: "response_flags"
    regex: "(response_flags=\\.=(.+?);\\.;)"
  - tag_name: "connection_security_policy"
    regex: "(connection_security_policy=\\.=(.+?);\\.;)"
  - tag_name: "permissive_response_code"
    regex: "(permissive_response_code=\\.=(.+?);\\.;)"
  - tag_name: "permissive_response_policyid"
    regex: "(permissive_response_policyid=\\.=(.+?);\\.;)"
  - tag_name: "cache"
    regex: "(cache\\.(.+?)\\.)"
  - tag_name: "component"
    regex: "(component\\.(.+?)\\.)"
  - tag_name: "tag"
    regex: "(tag\\.(.+?);\\.)"`

const metadataExchangeIstioUpstreamConfigFilterChain = `
filters:
- name: envoy.filters.network.upstream.metadata_exchange
  typed_config: 
    "@type": type.googleapis.com/envoy.tcp.metadataexchange.config.MetadataExchange
    protocol: istio2
`

const metadataExchangeIstioClientFilter = `
- name: envoy.filters.network.wasm
  config:
    config:
      root_id: "stats_outbound"
      vm_config:
        runtime: envoy.wasm.runtime.null
        code:
          local: { inline_string: "envoy.wasm.stats" }
      configuration: |
        { "debug": "false", max_peer_cache_size: 20, field_separator: ";.;" }
`

const tlsContext = `
tls_context:
  common_tls_context:
    alpn_protocols:
    - istio2
    tls_certificates:
    - certificate_chain: { filename: "testdata/certs/cert-chain.pem" }
      private_key: { filename: "testdata/certs/key.pem" }
    validation_context:
      trusted_ca: { filename: "testdata/certs/root-cert.pem" }
  require_client_certificate: true
`

const clusterTLSContext = `
tls_context:
  common_tls_context:
    alpn_protocols:
    - istio2
    tls_certificates:
    - certificate_chain: { filename: "testdata/certs/cert-chain.pem" }
      private_key: { filename: "testdata/certs/key.pem" }
    validation_context:
      trusted_ca: { filename: "testdata/certs/root-cert.pem" }
`

const clientNodeMetadata = `"NAMESPACE": "default",
"INCLUDE_INBOUND_PORTS": "9080",
"app": "productpage",
"EXCHANGE_KEYS": "NAME,NAMESPACE,INSTANCE_IPS,LABELS,OWNER,PLATFORM_METADATA,WORKLOAD_NAME,CANONICAL_TELEMETRY_SERVICE,MESH_ID,SERVICE_ACCOUNT",
"INSTANCE_IPS": "10.52.0.34,fe80::a075:11ff:fe5e:f1cd",
"pod-template-hash": "84975bc778",
"INTERCEPTION_MODE": "REDIRECT",
"SERVICE_ACCOUNT": "bookinfo-productpage",
"CONFIG_NAMESPACE": "default",
"version": "v1",
"OWNER": "kubernetes://apis/apps/v1/namespaces/default/deployments/productpage-v1",
"WORKLOAD_NAME": "productpage-v1",
"ISTIO_VERSION": "1.3-dev",
"kubernetes.io/limit-ranger": "LimitRanger plugin set: cpu request for container productpage",
"POD_NAME": "productpage-v1-84975bc778-pxz2w",
"istio": "sidecar",
"PLATFORM_METADATA": {
 "gcp_cluster_name": "test-cluster",
 "gcp_project": "test-project",
 "gcp_cluster_location": "us-east4-b"
},
"LABELS": {
 "app": "productpage",
 "version": "v1",
 "pod-template-hash": "84975bc778"
},
"ISTIO_PROXY_SHA": "istio-proxy:47e4559b8e4f0d516c0d17b233d127a3deb3d7ce",
"NAME": "productpage-v1-84975bc778-pxz2w",`

const serverNodeMetadata = `"NAMESPACE": "default",
"INCLUDE_INBOUND_PORTS": "9080",
"app": "ratings",
"EXCHANGE_KEYS": "NAME,NAMESPACE,INSTANCE_IPS,LABELS,OWNER,PLATFORM_METADATA,WORKLOAD_NAME,CANONICAL_TELEMETRY_SERVICE,MESH_ID,SERVICE_ACCOUNT",
"INSTANCE_IPS": "10.52.0.34,fe80::a075:11ff:fe5e:f1cd",
"pod-template-hash": "84975bc778",
"INTERCEPTION_MODE": "REDIRECT",
"SERVICE_ACCOUNT": "bookinfo-ratings",
"CONFIG_NAMESPACE": "default",
"version": "v1",
"OWNER": "kubernetes://apis/apps/v1/namespaces/default/deployments/ratings-v1",
"WORKLOAD_NAME": "ratings-v1",
"ISTIO_VERSION": "1.3-dev",
"kubernetes.io/limit-ranger": "LimitRanger plugin set: cpu request for container ratings",
"POD_NAME": "ratings-v1-84975bc778-pxz2w",
"istio": "sidecar",
"PLATFORM_METADATA": {
 "gcp_cluster_name": "test-cluster",
 "gcp_project": "test-project",
 "gcp_cluster_location": "us-east4-b"
},
"LABELS": {
 "app": "ratings",
 "version": "v1",
 "pod-template-hash": "84975bc778"
},
"ISTIO_PROXY_SHA": "istio-proxy:47e4559b8e4f0d516c0d17b233d127a3deb3d7ce",
"NAME": "ratings-v1-84975bc778-pxz2w",`

// Stats in Client Envoy proxy.
var expectedClientStats = map[string]int{
	"cluster.client.metadata_exchange.alpn_protocol_found":      1,
	"cluster.client.metadata_exchange.alpn_protocol_not_found":  0,
	"cluster.client.metadata_exchange.initial_header_not_found": 0,
	"cluster.client.metadata_exchange.header_not_found":         0,
	"cluster.client.metadata_exchange.metadata_added":           1,
}

// Stats in Server Envoy proxy.
var expectedPrometheusServerLabels = map[string]string{
	"reporter":        "destination",
	"source_app":      "productpage",
	"destination_app": "ratings",
}
var expectedPrometheusServerStats = map[string]env.Stat{
	"istio_requests_total": {Value: 1, Labels: expectedPrometheusServerLabels},
}

// Stats in Server Envoy proxy.
var expectedServerStats = map[string]int{
	"metadata_exchange.alpn_protocol_found":      1,
	"metadata_exchange.alpn_protocol_not_found":  0,
	"metadata_exchange.initial_header_not_found": 0,
	"metadata_exchange.header_not_found":         0,
	"metadata_exchange.metadata_added":           1,
}

func TestTCPMetadataExchange(t *testing.T) {
	s := env.NewClientServerEnvoyTestSetup(env.TCPMetadataExchangeTest, t)
	s.Dir = driver.BazelWorkspace()
	s.SetNoBackend(true)
	s.SetStartTCPBackend(true)
	s.SetTLSContext(tlsContext)
	s.SetClusterTLSContext(clusterTLSContext)
	s.SetFiltersBeforeEnvoyRouterInProxyToServer(metadataExchangeIstioConfigFilter)
	s.SetUpstreamFiltersInClient(metadataExchangeIstioUpstreamConfigFilterChain)
	s.SetFiltersBeforeEnvoyRouterInAppToClient(metadataExchangeIstioClientFilter)
	s.SetEnableTLS(true)
	s.SetClientNodeMetadata(clientNodeMetadata)
	s.SetServerNodeMetadata(serverNodeMetadata)
	s.SetExtraConfig(statsConfig)
	s.ClientEnvoyTemplate = env.GetTCPClientEnvoyConfTmp()
	s.ServerEnvoyTemplate = env.GetTCPServerEnvoyConfTmp()
	if err := s.SetUpClientServerEnvoy(); err != nil {
		t.Fatalf("Failed to setup te1	st: %v", err)
	}
	defer s.TearDownClientServerEnvoy()

	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(driver.TestPath("testdata/certs/cert-chain.pem"))
	if err != nil {
		t.Fatalf("failed to read client ca cert: %s", err)
	}
	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		t.Fatal("failed to append client certs")
	}

	certificate, err := tls.LoadX509KeyPair(driver.TestPath("testdata/certs/cert-chain.pem"),
		driver.TestPath("testdata/certs/key.pem"))
	if err != nil {
		t.Fatal("failed to get certificate")
	}
	config := &tls.Config{Certificates: []tls.Certificate{certificate}, ServerName: "localhost", NextProtos: []string{"istio2"}, RootCAs: certPool}

	conn, err := tls.Dial("tcp", fmt.Sprintf("localhost:%d", s.Ports().AppToClientProxyPort), config)
	if err != nil {
		t.Fatal(err)
	}

	conn.Write([]byte("world \n"))
	reply := make([]byte, 256)
	n, err := conn.Read(reply)
	if err != nil {
		t.Fatal(err)
	}

	if fmt.Sprintf("%s", reply[:n]) != "hello world \n" {
		t.Fatalf("Verification Failed. Expected: hello world. Got: %v", fmt.Sprintf("%s", reply[:n]))
	}

	_ = conn.Close()
	s.VerifyEnvoyStats(getParsedExpectedStats(expectedClientStats, t, s), s.Ports().ClientAdminPort)
	s.VerifyEnvoyStats(getParsedExpectedStats(expectedServerStats, t, s), s.Ports().ServerAdminPort)

	s.VerifyPrometheusStats(expectedPrometheusServerStats, s.Ports().ServerAdminPort)

}

func getParsedExpectedStats(expectedStats map[string]int, t *testing.T, s *env.TestSetup) map[string]int {
	parsedExpectedStats := make(map[string]int)
	for key, value := range expectedStats {
		tmpl, err := template.New("parse_state").Parse(key)
		if err != nil {
			t.Errorf("failed to parse config template: %v", err)
		}

		var tpl bytes.Buffer
		err = tmpl.Execute(&tpl, s)
		if err != nil {
			t.Errorf("failed to execute config template: %v", err)
		}
		parsedExpectedStats[tpl.String()] = value
	}

	return parsedExpectedStats
}
