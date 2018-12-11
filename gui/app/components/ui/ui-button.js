// Copyright 2016 Documize Inc. <legal@documize.com>. All rights reserved.
//
// This software (Documize Community Edition) is licensed under
// GNU AGPL v3 http://www.gnu.org/licenses/agpl-3.0.en.html
//
// You can operate outside the AGPL restrictions by purchasing
// Documize Enterprise Edition and obtaining a commercial license
// by contacting <sales@documize.com>.
//
// https://documize.com

import { computed } from '@ember/object';
import Component from '@ember/component';

export default Component.extend({
	tagName: 'button',
	classNames: [],
	classNameBindings: ['calcClass'],
	attributeBindings: ['calcAttrs:data-dismiss'],

	label: '',
	icon: '',
	color: '',
	light: false,
	themed: false,
	dismiss: false,

	iconClass: '',
	hasIcon: computed('iconClass', function() {
		return this.iconClass.trim() != '';
	}),

	calcClass: computed(function() {
		// Prepare icon class name
		this.iconClass = this.icon;

		// Prepare button class name
		let bc = 'dmz-button';
		if (this.themed) {
			bc += '-theme';
		} else {
			bc += '-' + this.color;
		}

		if (this.light) {
			bc += '-light';
		}

		return bc;
	}),

	calcAttrs: computed(function() {
		if (this.dismiss) {
			return 'modal';
		}

		return null;
	}),

	click(e) {
		if (is.not.undefined(this.onClick)) {
			this.onClick(e);
		}
	}
});