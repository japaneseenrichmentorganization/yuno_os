/* Yuno OS Calamares Slideshow */

import QtQuick 2.0;
import calamares.slideshow 1.0;

Presentation
{
    id: presentation

    Timer {
        interval: 20000
        running: true
        repeat: true
        onTriggered: presentation.goToNextSlide()
    }

    Slide {
        Image {
            id: background1
            source: "images/slide1.png"
            width: parent.width
            height: parent.height
            fillMode: Image.PreserveAspectFit
            anchors.centerIn: parent
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 50
            text: "Welcome to Yuno OS"
            color: "#FFFFFF"
            font.pixelSize: 24
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 20
            text: "A Gentoo-based distribution with an easy installer"
            color: "#CCCCCC"
            font.pixelSize: 16
        }
    }

    Slide {
        Text {
            anchors.centerIn: parent
            text: "Installing Gentoo Made Easy"
            color: "#FFFFFF"
            font.pixelSize: 28
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.verticalCenter
            anchors.topMargin: 40
            text: "• Automatic partitioning\n• Pre-configured profiles\n• Binary package support\n• Full customization options"
            color: "#CCCCCC"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }
    }

    Slide {
        Text {
            anchors.centerIn: parent
            text: "Powerful Customization"
            color: "#FFFFFF"
            font.pixelSize: 28
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.verticalCenter
            anchors.topMargin: 40
            text: "• Custom CFLAGS presets\n• LTO optimization overlay\n• Multiple kernel options\n• Choose your init system"
            color: "#CCCCCC"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }
    }

    Slide {
        Text {
            anchors.centerIn: parent
            text: "Desktop Environments"
            color: "#FFFFFF"
            font.pixelSize: 28
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.verticalCenter
            anchors.topMargin: 40
            text: "• KDE Plasma\n• GNOME\n• XFCE, LXQt, Cinnamon\n• i3, Sway, Hyprland"
            color: "#CCCCCC"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }
    }

    Slide {
        Text {
            anchors.centerIn: parent
            text: "Security & Privacy"
            color: "#FFFFFF"
            font.pixelSize: 28
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.verticalCenter
            anchors.topMargin: 40
            text: "• Full disk encryption (LUKS)\n• Secure Boot support\n• Hardened kernel options\n• Regular security updates"
            color: "#CCCCCC"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }
    }

    Slide {
        Text {
            anchors.centerIn: parent
            text: "Getting Started"
            color: "#FFFFFF"
            font.pixelSize: 28
            font.bold: true
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.verticalCenter
            anchors.topMargin: 40
            text: "After installation:\n\n1. Update packages: emerge --sync\n2. Install software: emerge <package>\n3. Read the Gentoo Wiki\n4. Join the community!"
            color: "#CCCCCC"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }
    }
}
