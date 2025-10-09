package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/stage1.bin#e5d4d654ca1fa2c388bf64b23fc6e67815893fc7cb8b7cfee253d87963f54973",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/stage2-basic.tar.bz2#9a2b4d71e9779801bde73c16b3be789bc50672019a87e8c90fe3c94e034907c1",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/stage1.bin#e5d4d654ca1fa2c388bf64b23fc6e67815893fc7cb8b7cfee253d87963f54973",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.6.2/stage2-podman.tar.bz2#b2ea2a0ca769b6b2d64e3f0c577ee9c08f0bb81a6e33ed5b15b2a7e50ef9a09f",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.8.1/rofl-containers#993e66f3b037cdc63896216cde4dc91b4976cc82a698c63123a9bba8710852c4",
	},
}
