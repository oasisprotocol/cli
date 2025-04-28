package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/stage1.bin#06e12cba9b2423b4dd5916f4d84bf9c043f30041ab03aa74006f46ef9c129d22",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/stage2-basic.tar.bz2#72c84d2566959799fdd98fae08c143a8572a5a09ee426be376f9a8bbd1675f2b",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/stage1.bin#06e12cba9b2423b4dd5916f4d84bf9c043f30041ab03aa74006f46ef9c129d22",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.1/stage2-podman.tar.bz2#6f2487aa064460384309a58c858ffea9316e739331b5c36789bb2f61117869d6",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.5.0/rofl-containers#800be74e543f1d10d12ef6fadce89dd0a0ce7bc798dbab4f8d7aa012d82fbff1",
		Compose: "compose.yaml",
	},
}
