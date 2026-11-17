package rofl

// Builder images for different app kinds.
const (
	// LatestBuilderImage is the full builder with Rust toolchain for raw apps.
	LatestBuilderImage = "ghcr.io/oasisprotocol/rofl-dev:v0.7.0@sha256:afc6d1baa1ad8df4031baa30ac627b86cdc5b726e62c69ddf7de30326637392f"
	// LatestContainerBuilderImage is the minimal builder for container apps.
	LatestContainerBuilderImage = "ghcr.io/oasisprotocol/rofl-container-builder:0.0.1@sha256:913ef97ab07dde31f08ce873f825bf3d4f32ad4102ff5797d7c3050c121c4dce"
)

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Platform: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/ovmf.tdx.platform.json.gz#0c61429b8cd69566cb32abda134a4e4501941a625423a6801078ebff47325ad0",
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/ovmf.tdx.fd#cb3675d68f450dcc95861c6360181a63dc1e7fa176cc23bfca26fd689663d6bc",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/stage1.bin#0658d197ddbac905c07c1e90b47b51efef54c72d27ba5589bb05ae59446fa6cc",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/stage2-basic.tar.bz2#9a2b4d71e9779801bde73c16b3be789bc50672019a87e8c90fe3c94e034907c1",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Platform: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/ovmf.tdx.platform.json.gz#0c61429b8cd69566cb32abda134a4e4501941a625423a6801078ebff47325ad0",
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/ovmf.tdx.fd#cb3675d68f450dcc95861c6360181a63dc1e7fa176cc23bfca26fd689663d6bc",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/stage1.bin#0658d197ddbac905c07c1e90b47b51efef54c72d27ba5589bb05ae59446fa6cc",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.7.0/stage2-podman.tar.bz2#e102a39395363cf178f9e5c2cc2f4d78623dba955647a6073c9298bf09c9c1ff",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.11.0/rofl-containers#e39dd80f6ec02e2b579a12571cd7b5e460a29b6b119377c1e529e8e909a8db72",
	},
}
