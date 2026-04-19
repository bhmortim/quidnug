// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Quidnug",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [
        .library(name: "Quidnug", targets: ["Quidnug"]),
    ],
    dependencies: [],
    targets: [
        .target(name: "Quidnug"),
        .testTarget(name: "QuidnugTests", dependencies: ["Quidnug"]),
    ]
)
