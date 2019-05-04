/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import Foundation
import MozillaAppServices
import AuthenticationServices

extension LoginRecord: Equatable {
    public static func == (lhs: LoginRecord, rhs: LoginRecord) -> Bool {
        return lhs.id == rhs.id &&
            lhs.username == rhs.username &&
            rhs.password == rhs.password
    }
}

@available(iOS 12, *)
extension LoginRecord {
    open var passwordCredentialIdentity: ASPasswordCredentialIdentity {
        let serviceIdentifier = ASCredentialServiceIdentifier(identifier: self.hostname, type: .URL)
        return ASPasswordCredentialIdentity(serviceIdentifier: serviceIdentifier, user: self.username ?? "", recordIdentifier: self.id)
    }

    open var passwordCredential: ASPasswordCredential {
        return ASPasswordCredential(user: self.username ?? "", password: self.password)
    }
}
