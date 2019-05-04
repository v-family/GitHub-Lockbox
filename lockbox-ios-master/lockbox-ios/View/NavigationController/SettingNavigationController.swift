/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import UIKit

class SettingNavigationController: UINavigationController {
    convenience init() {
        let settingListView = UIStoryboard(name: "SettingList", bundle: .main)
                .instantiateViewController(withIdentifier: "settinglist")
        self.init(rootViewController: settingListView)
        self.view.backgroundColor = UIColor.white
    }
}
