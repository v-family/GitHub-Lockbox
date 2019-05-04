/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import Foundation
import MozillaAppServices

struct SyncCredential {
    let syncInfo: SyncUnlockInfo
    let isNew: Bool
}

let OfflineSyncCredential = SyncCredential(
    syncInfo: SyncUnlockInfo(kid: "", fxaAccessToken: "", syncKey: "", tokenserverURL: ""),
    isNew: false
)

extension SyncCredential: Equatable {
    public static func ==(lhs: SyncCredential, rhs: SyncCredential) -> Bool {
        return lhs.isNew == rhs.isNew &&
            lhs.syncInfo == rhs.syncInfo
    }
}
