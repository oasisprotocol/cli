package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/stage1.bin#06e12cba9b2423b4dd5916f4d84bf9c043f30041ab03aa74006f46ef9c129d22",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/stage2-basic.tar.bz2#72c84d2566959799fdd98fae08c143a8572a5a09ee426be376f9a8bbd1675f2b",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/stage1.bin#06e12cba9b2423b4dd5916f4d84bf9c043f30041ab03aa74006f46ef9c129d22",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.0/stage2-podman.tar.bz2#827531546f3db6b0945ece7ddab4e10d648eaa3ba1c146b7889d7cb9cbf0b507",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.4.1/rofl-containers#bdd2735af9ff10c9b1c1e8db535f4751739bd3707600c57b81e80195e6207673",
		Compose: "compose.yaml",
	},
}
