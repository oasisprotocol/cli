package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/stage1.bin#02903bd0ddfe1e3552e95767f1be17e801690d73d90bb1e800aa4879ba46c4d7",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/stage2-basic.tar.bz2#72c84d2566959799fdd98fae08c143a8572a5a09ee426be376f9a8bbd1675f2b",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/stage1.bin#02903bd0ddfe1e3552e95767f1be17e801690d73d90bb1e800aa4879ba46c4d7",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.4.2/stage2-podman.tar.bz2#6f2487aa064460384309a58c858ffea9316e739331b5c36789bb2f61117869d6",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.5.1/rofl-containers#9afa712b939528d758294bf49181466fc2066bbe507f92777ddc3bce8af6ee37",
		Compose: "compose.yaml",
	},
}
