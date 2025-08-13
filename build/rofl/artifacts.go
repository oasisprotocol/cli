package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/stage1.bin#118be803795bb40c991bc4d74577c17c04fbdb985106f5a91e23f84e8c2f36b3",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/stage2-basic.tar.bz2#9a2b4d71e9779801bde73c16b3be789bc50672019a87e8c90fe3c94e034907c1",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/stage1.bin#118be803795bb40c991bc4d74577c17c04fbdb985106f5a91e23f84e8c2f36b3",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.1/stage2-podman.tar.bz2#b2ea2a0ca769b6b2d64e3f0c577ee9c08f0bb81a6e33ed5b15b2a7e50ef9a09f",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.7.0/rofl-containers#d9e80fcd85534a3005f2d7d2d9231ae32f49f8ad26363f5e87b2abcfb5274722",
		Compose: "compose.yaml",
	},
}
