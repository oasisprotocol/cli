package rofl

// LatestBasicArtifacts are the latest TDX ROFL basic app artifacts.
var LatestBasicArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/stage1.bin#23877530413a661e9187aad2eccfc9660fc4f1a864a1fbad2f6c7d43512071ca",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/stage2-basic.tar.bz2#72c84d2566959799fdd98fae08c143a8572a5a09ee426be376f9a8bbd1675f2b",
}

// LatestContainerArtifacts are the latest TDX container app artifacts.
var LatestContainerArtifacts = ArtifactsConfig{
	Firmware: "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	Kernel:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/stage1.bin#23877530413a661e9187aad2eccfc9660fc4f1a864a1fbad2f6c7d43512071ca",
	Stage2:   "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.5.0/stage2-podman.tar.bz2#631349bef06990dd6ae882812a0420f4b35f87f9fe945b274bcfb10fc08c4ea3",
	Container: ContainerArtifactsConfig{
		Runtime: "https://github.com/oasisprotocol/oasis-sdk/releases/download/rofl-containers%2Fv0.5.2/rofl-containers#3abac3e7519588c1a4e250f1bf5cbbbd58c4e4c8ee817070031b9e0e3d4e0095",
		Compose: "compose.yaml",
	},
}
