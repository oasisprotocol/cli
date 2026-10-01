package rofl

// Builder images for different app kinds.
const (
	// LatestBuilderImage is the full builder with Rust toolchain for raw apps.
	LatestBuilderImage = "ghcr.io/oasisprotocol/rofl-dev:v0.5.0@sha256:31573686552abeb0edebc450f6872831f0006a6cf38220cef7e0789d4376c2c1"
	// LatestContainerBuilderImage is the minimal builder for container apps.
	LatestContainerBuilderImage = "ghcr.io/oasisprotocol/rofl-container-builder:0.0.1@sha256:913ef97ab07dde31f08ce873f825bf3d4f32ad4102ff5797d7c3050c121c4dce"
)

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/stage1.bin#2a39f1e317f217e3e46e036383d6716c199d89e58d14901251e54c195e5261f0",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/stage2-basic.tar.bz2#9a2b4d71e9779801bde73c16b3be789bc50672019a87e8c90fe3c94e034907c1",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/stage1.bin#2a39f1e317f217e3e46e036383d6716c199d89e58d14901251e54c195e5261f0",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.3/stage2-podman.tar.bz2#e102a39395363cf178f9e5c2cc2f4d78623dba955647a6073c9298bf09c9c1ff",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.9.0/rofl-containers#e2e074d03ab2fbacaacb01e6d63ce093fde04e4cc620dd8804e8afbb20390525",
	},
}
