package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/stage1.bin#539f25c66a27b2ca3c6b4d3333b88c64e531fcc96776c37a12c9ce06dd7fbac9",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/stage2-basic.tar.bz2#72c84d2566959799fdd98fae08c143a8572a5a09ee426be376f9a8bbd1675f2b",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/stage1.bin#539f25c66a27b2ca3c6b4d3333b88c64e531fcc96776c37a12c9ce06dd7fbac9",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.3.3/stage2-podman.tar.bz2#827531546f3db6b0945ece7ddab4e10d648eaa3ba1c146b7889d7cb9cbf0b507",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.3.4/rofl-containers#d6a055b2e88e1f321e3ab1f73046444e24df9d8925d13cc6b8230de9a81e5c41",
		Compose: "compose.yaml",
	},
}
