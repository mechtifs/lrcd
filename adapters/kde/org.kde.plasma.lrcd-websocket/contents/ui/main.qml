pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import QtWebSockets
import org.kde.kirigami as Kirigami
import org.kde.plasma.plasmoid

PlasmoidItem {
    id: root
    preferredRepresentation: fullRepresentation
    fullRepresentation: Item {
        Layout.preferredWidth:  Math.max(label.implicitWidth, 300)
        Layout.preferredHeight: label.implicitHeight

        Label {
            id: label
            anchors.fill: parent
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            color: Kirigami.Theme.textColor
        }

        Loader {
            id: socketLoader
            sourceComponent: wsComponent
            onStatusChanged: if (status === Loader.Null) {
                reconnectTimer.start()
            }
        }

        Component {
            id: wsComponent
            WebSocket {
                url: "ws://127.0.0.1:5723"
                active: true

                onTextMessageReceived: (msg) => label.text = msg < " " ? "" : msg

                onStatusChanged: (st) => {
                    switch (st) {
                    case WebSocket.Connecting:
                        reconnectTimer.stop()
                        break
                    case WebSocket.Closing:
                        label.text = ""
                        break
                    case WebSocket.Closed:
                        socketLoader.sourceComponent = undefined
                        break
                    }
                }
            }
        }

        Timer {
            id: reconnectTimer
            interval: 1000
            repeat: false
            onTriggered: socketLoader.sourceComponent = wsComponent
        }
    }
}
