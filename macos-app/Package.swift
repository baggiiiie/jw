// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "JWMenu",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "JWMenu",
            path: "Sources/JWMenu"
        ),
    ]
)
