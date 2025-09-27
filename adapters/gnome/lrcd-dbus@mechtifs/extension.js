import Clutter from 'gi://Clutter';
import Gio from 'gi://Gio';
import St from 'gi://St';

import { Extension, gettext as _ } from 'resource:///org/gnome/shell/extensions/extension.js';

import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';

export default class LrcdDBusExtension extends Extension {
	enable() {
		this._indicator = new PanelMenu.Button(0.0, this.metadata.name, true);
		this._indicator.hide();

		const label = new St.Label({ style_class: 'panel-button', y_align: Clutter.ActorAlign.CENTER, y_expand: true });
		this._indicator.add_child(label);

		Main.panel.addToStatusArea('lrcd-indicator', this._indicator, -1, 'left');

		this._handlerId = Gio.DBus.session.signal_subscribe(null, 'com.github.mechtifs.lrcd', 'Updated', '/com/github/mechtifs/lrcd', null, Gio.DBusSignalFlags.NONE, (connection, sender, path, iface, signal, params) => {
			const line = params.unpack()[0].get_string()[0];

			if (line < ' ') {
				this._indicator.hide();
			} else {
				this._indicator.show();
				label.text = line;
			}
		});
	}

	disable() {
		Gio.DBus.session.signal_unsubscribe(this._handlerId);
		this._indicator.destroy();
		delete this._indicator;
	}
}
