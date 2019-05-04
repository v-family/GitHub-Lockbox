/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import Quick
import Nimble
import RxSwift
import RxTest
import RxBlocking
import MozillaAppServices
import SwiftKeychainWrapper
import WebKit
import Reachability

@testable import Lockbox

class AccountStoreSpec: QuickSpec {
    class FakeDispatcher: Dispatcher {
        let fakeRegistration = PublishSubject<Action>()

        override var register: Observable<Action> {
            return self.fakeRegistration.asObservable()
        }
    }

    class FakeKeychainManager: KeychainWrapper {
        var saveArguments: [String: String] = [:]
        var saveSuccess: Bool!
        var retrieveResult: [String: String] = [:]
        var removeArguments: [String] = []

        override func set(_ value: String, forKey key: String, withAccessibility accessibility: SwiftKeychainWrapper.KeychainItemAccessibility? = nil) -> Bool {
            self.saveArguments[key] = value
            return saveSuccess
        }

        override func string(forKey key: String, withAccessibility accessibility: KeychainItemAccessibility? = nil) -> String? {
            return retrieveResult[key]
        }

        override func removeObject(forKey key: String, withAccessibility accessibility: KeychainItemAccessibility?) -> Bool {
            self.removeArguments.append(key)
            return true
        }

        init() { super.init(serviceName: "blah") }
    }

    class FakeURLCache: URLCache {
        var removeCachedResponsesCalled = false

        override func removeAllCachedResponses() {
            self.removeCachedResponsesCalled = true
        }
    }

    class FakeReachability: ReachabilityProtocol {
        var connection: Reachability.Connection {
            return .none
        }
        
        var whenReachable: Reachability.NetworkReachable?
        var whenUnreachable: Reachability.NetworkUnreachable?
        func startNotifier() {
        }
    }

    public class FakeNetworkStore: NetworkStore {
        init() {
            super.init(reachability: FakeReachability())
        }
        var connected: Bool = false

        override var isConnectedToNetwork: Bool {
            return connected
        }
    }

    private var dispatcher: FakeDispatcher!
    private var keychainManager: FakeKeychainManager!
    private var networkStore: FakeNetworkStore!
    private var urlCache: FakeURLCache!
    private var scheduler = TestScheduler(initialClock: 0)
    private var disposeBag = DisposeBag()
    var subject: AccountStore!

    override func spec() {
        describe("AccountStore") {
            beforeEach {
                self.dispatcher = FakeDispatcher()
                self.keychainManager = FakeKeychainManager()
                self.networkStore = FakeNetworkStore()
                self.urlCache = FakeURLCache()
                self.keychainManager.saveSuccess = true

                self.subject = AccountStore(
                        dispatcher: self.dispatcher,
                        networkStore: self.networkStore,
                        keychainWrapper: self.keychainManager,
                        urlCache: self.urlCache
                )
            }

            describe("loginURL") {
                it("populates the loginURL for the Lockbox configuration on initialization") {

                }
            }

            xdescribe("profile") {
                describe("when the shared keychain has a valid fxa account") {
                    beforeEach {
                        self.keychainManager.retrieveResult[KeychainKey.accountJSON.rawValue] = "{\"schema_version\":\"V1\",\"client_id\":\"98adfa37698f255b\",\"redirect_uri\":\"https://lockbox.firefox.com/fxa/ios-redirect.html\",\"config\":{\"content_url\":\"https://accounts.firefox.com\",\"auth_url\":\"https://api.accounts.firefox.com/\",\"oauth_url\":\"https://oauth.accounts.firefox.com/\",\"profile_url\":\"https://profile.accounts.firefox.com/\",\"token_server_endpoint_url\":\"https://token.services.mozilla.com/1.0/sync/1.5\",\"authorization_endpoint\":\"https://accounts.firefox.com/authorization\",\"issuer\":\"https://accounts.firefox.com\",\"jwks_uri\":\"https://oauth.accounts.firefox.com/v1/jwks\",\"token_endpoint\":\"https://oauth.accounts.firefox.com/v1/token\",\"userinfo_endpoint\":\"https://profile.accounts.firefox.com/v1/profile\"},\"oauth_cache\":{\"https://identity.mozilla.com/apps/oldsync https://identity.mozilla.com/apps/lockbox profile\":{\"access_token\":\"abd1a1e02fc7afa5ddcba9e5d768297e2c883ff3926ee075bca226067a944685\",\"keys\":\"{\\\"https://identity.mozilla.com/apps/oldsync\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/oldsync\\\",\\\"k\\\":\\\"VEZDYJ3Jd1Ui0ZVtW8pPHD6LZ48Jd30p-y-PLQQYa0PRcMZtiM6zJO4_I2lxEg__qkxXldPyLiM5PYY9VBD64w\\\",\\\"kid\\\":\\\"1519160140602-WMF1HOhJbtMVueuy3tV4vA\\\"},\\\"https://identity.mozilla.com/apps/lockbox\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/lockbox\\\",\\\"k\\\":\\\"oGGfsZk8xMXtBzGzy2WY3QGPNOTer0VGIC3Uyz9Jy9w\\\",\\\"kid\\\":\\\"1519160141-YqmShzWPQhHp0RNiZs25zg\\\"}}\",\"refresh_token\":\"2b5a070455ba24cdc2ce7bb7ce43aef5b6e0b28bc6cd76b0083a50604e1bba00\",\"expires_at\":1533664155,\"scopes\":[\"https://identity.mozilla.com/apps/oldsync\",\"https://identity.mozilla.com/apps/lockbox\",\"profile\"]}}}"

                        self.subject = AccountStore(
                                dispatcher: self.dispatcher,
                                networkStore: self.networkStore,
                                keychainWrapper: self.keychainManager
                        )
                    }

                    it("pushes a non-nil profile") {
                        // can't check anything more detailed because we can't construct FxAClient.Profile
                        let profile = try! self.subject.profile.toBlocking().first()
                        expect(profile).notTo(beNil())
                    }
                }

                describe("when the shared keychain does not have a valid fxa account but the local keychain does") {
                    beforeEach {
                        self.keychainManager.retrieveResult[KeychainKey.accountJSON.rawValue] = "{\"schema_version\":\"V1\",\"client_id\":\"98adfa37698f255b\",\"redirect_uri\":\"https://lockbox.firefox.com/fxa/ios-redirect.html\",\"config\":{\"content_url\":\"https://accounts.firefox.com\",\"auth_url\":\"https://api.accounts.firefox.com/\",\"oauth_url\":\"https://oauth.accounts.firefox.com/\",\"profile_url\":\"https://profile.accounts.firefox.com/\",\"token_server_endpoint_url\":\"https://token.services.mozilla.com/1.0/sync/1.5\",\"authorization_endpoint\":\"https://accounts.firefox.com/authorization\",\"issuer\":\"https://accounts.firefox.com\",\"jwks_uri\":\"https://oauth.accounts.firefox.com/v1/jwks\",\"token_endpoint\":\"https://oauth.accounts.firefox.com/v1/token\",\"userinfo_endpoint\":\"https://profile.accounts.firefox.com/v1/profile\"},\"oauth_cache\":{\"https://identity.mozilla.com/apps/oldsync https://identity.mozilla.com/apps/lockbox profile\":{\"access_token\":\"abd1a1e02fc7afa5ddcba9e5d768297e2c883ff3926ee075bca226067a944685\",\"keys\":\"{\\\"https://identity.mozilla.com/apps/oldsync\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/oldsync\\\",\\\"k\\\":\\\"VEZDYJ3Jd1Ui0ZVtW8pPHD6LZ48Jd30p-y-PLQQYa0PRcMZtiM6zJO4_I2lxEg__qkxXldPyLiM5PYY9VBD64w\\\",\\\"kid\\\":\\\"1519160140602-WMF1HOhJbtMVueuy3tV4vA\\\"},\\\"https://identity.mozilla.com/apps/lockbox\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/lockbox\\\",\\\"k\\\":\\\"oGGfsZk8xMXtBzGzy2WY3QGPNOTer0VGIC3Uyz9Jy9w\\\",\\\"kid\\\":\\\"1519160141-YqmShzWPQhHp0RNiZs25zg\\\"}}\",\"refresh_token\":\"2b5a070455ba24cdc2ce7bb7ce43aef5b6e0b28bc6cd76b0083a50604e1bba00\",\"expires_at\":1533664155,\"scopes\":[\"https://identity.mozilla.com/apps/oldsync\",\"https://identity.mozilla.com/apps/lockbox\",\"profile\"]}}}"

                        self.subject = AccountStore(
                            dispatcher: self.dispatcher,
                            networkStore: self.networkStore,
                            keychainWrapper: self.keychainManager
                        )
                    }

                    it("pushes a non-nil profile") {
                        // can't check anything more detailed because we can't construct FxAClient.Profile
                        let profile = try! self.subject.profile.toBlocking().first()
                        expect(profile).notTo(beNil())
                    }
                }

                describe("when neither the local nor the shared keychain have a valid fxa account") {
                    it("pushes a nil profile") {
                        let profile = try! self.subject.profile.toBlocking().first()!
                        expect(profile).to(beNil())
                    }
                }
            }

            xdescribe("syncCredentials") {
                describe("when the shared keychain has a valid fxa account") {
                    beforeEach {
                        self.keychainManager.retrieveResult[KeychainKey.accountJSON.rawValue] = "{\"schema_version\":\"V1\",\"client_id\":\"98adfa37698f255b\",\"redirect_uri\":\"https://lockbox.firefox.com/fxa/ios-redirect.html\",\"config\":{\"content_url\":\"https://accounts.firefox.com\",\"auth_url\":\"https://api.accounts.firefox.com/\",\"oauth_url\":\"https://oauth.accounts.firefox.com/\",\"profile_url\":\"https://profile.accounts.firefox.com/\",\"token_server_endpoint_url\":\"https://token.services.mozilla.com/1.0/sync/1.5\",\"authorization_endpoint\":\"https://accounts.firefox.com/authorization\",\"issuer\":\"https://accounts.firefox.com\",\"jwks_uri\":\"https://oauth.accounts.firefox.com/v1/jwks\",\"token_endpoint\":\"https://oauth.accounts.firefox.com/v1/token\",\"userinfo_endpoint\":\"https://profile.accounts.firefox.com/v1/profile\"},\"oauth_cache\":{\"https://identity.mozilla.com/apps/oldsync https://identity.mozilla.com/apps/lockbox profile\":{\"access_token\":\"abd1a1e02fc7afa5ddcba9e5d768297e2c883ff3926ee075bca226067a944685\",\"keys\":\"{\\\"https://identity.mozilla.com/apps/oldsync\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/oldsync\\\",\\\"k\\\":\\\"VEZDYJ3Jd1Ui0ZVtW8pPHD6LZ48Jd30p-y-PLQQYa0PRcMZtiM6zJO4_I2lxEg__qkxXldPyLiM5PYY9VBD64w\\\",\\\"kid\\\":\\\"1519160140602-WMF1HOhJbtMVueuy3tV4vA\\\"},\\\"https://identity.mozilla.com/apps/lockbox\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/lockbox\\\",\\\"k\\\":\\\"oGGfsZk8xMXtBzGzy2WY3QGPNOTer0VGIC3Uyz9Jy9w\\\",\\\"kid\\\":\\\"1519160141-YqmShzWPQhHp0RNiZs25zg\\\"}}\",\"refresh_token\":\"2b5a070455ba24cdc2ce7bb7ce43aef5b6e0b28bc6cd76b0083a50604e1bba00\",\"expires_at\":1533664155,\"scopes\":[\"https://identity.mozilla.com/apps/oldsync\",\"https://identity.mozilla.com/apps/lockbox\",\"profile\"]}}}"

                        self.subject = AccountStore(
                                dispatcher: self.dispatcher,
                                networkStore: self.networkStore,
                                keychainWrapper: self.keychainManager
                        )
                    }

                    it("pushes a non-nil oauthinfo") {
                        // can't check anything more detailed because we can't construct FxAClient.OAuthInfo
                        let syncInfo = try! self.subject.syncCredentials.toBlocking().first()
                        expect(syncInfo).notTo(beNil())
                    }

                    it("saves the refreshed JSON to the shared keychain after pushing the oauth info") {
                        _ = try! self.subject.syncCredentials.toBlocking().first()
                        expect(self.keychainManager.saveArguments[KeychainKey.accountJSON.rawValue]).notTo(beNil())
                    }
                }

                describe("when the local keychain has a valid fxa account") {
                    beforeEach {
                        self.keychainManager.retrieveResult[KeychainKey.accountJSON.rawValue] = "{\"schema_version\":\"V1\",\"client_id\":\"98adfa37698f255b\",\"redirect_uri\":\"https://lockbox.firefox.com/fxa/ios-redirect.html\",\"config\":{\"content_url\":\"https://accounts.firefox.com\",\"auth_url\":\"https://api.accounts.firefox.com/\",\"oauth_url\":\"https://oauth.accounts.firefox.com/\",\"profile_url\":\"https://profile.accounts.firefox.com/\",\"token_server_endpoint_url\":\"https://token.services.mozilla.com/1.0/sync/1.5\",\"authorization_endpoint\":\"https://accounts.firefox.com/authorization\",\"issuer\":\"https://accounts.firefox.com\",\"jwks_uri\":\"https://oauth.accounts.firefox.com/v1/jwks\",\"token_endpoint\":\"https://oauth.accounts.firefox.com/v1/token\",\"userinfo_endpoint\":\"https://profile.accounts.firefox.com/v1/profile\"},\"oauth_cache\":{\"https://identity.mozilla.com/apps/oldsync https://identity.mozilla.com/apps/lockbox profile\":{\"access_token\":\"abd1a1e02fc7afa5ddcba9e5d768297e2c883ff3926ee075bca226067a944685\",\"keys\":\"{\\\"https://identity.mozilla.com/apps/oldsync\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/oldsync\\\",\\\"k\\\":\\\"VEZDYJ3Jd1Ui0ZVtW8pPHD6LZ48Jd30p-y-PLQQYa0PRcMZtiM6zJO4_I2lxEg__qkxXldPyLiM5PYY9VBD64w\\\",\\\"kid\\\":\\\"1519160140602-WMF1HOhJbtMVueuy3tV4vA\\\"},\\\"https://identity.mozilla.com/apps/lockbox\\\":{\\\"kty\\\":\\\"oct\\\",\\\"scope\\\":\\\"https://identity.mozilla.com/apps/lockbox\\\",\\\"k\\\":\\\"oGGfsZk8xMXtBzGzy2WY3QGPNOTer0VGIC3Uyz9Jy9w\\\",\\\"kid\\\":\\\"1519160141-YqmShzWPQhHp0RNiZs25zg\\\"}}\",\"refresh_token\":\"2b5a070455ba24cdc2ce7bb7ce43aef5b6e0b28bc6cd76b0083a50604e1bba00\",\"expires_at\":1533664155,\"scopes\":[\"https://identity.mozilla.com/apps/oldsync\",\"https://identity.mozilla.com/apps/lockbox\",\"profile\"]}}}"

                        self.subject = AccountStore(
                            dispatcher: self.dispatcher,
                            networkStore: self.networkStore,
                            keychainWrapper: self.keychainManager
                        )
                    }

                    it("pushes a non-nil syncCredential") {
                        // can't check anything more detailed because we can't construct FxAClient.OAuthInfo
                        let syncInfo = try! self.subject.syncCredentials.toBlocking().first()
                        expect(syncInfo).notTo(beNil())
                    }

                    it("saves the refreshed JSON to the shared keychain") {
                        _ = try! self.subject.syncCredentials.toBlocking().first()
                        expect(self.keychainManager.saveArguments[KeychainKey.accountJSON.rawValue]).notTo(beNil())
                    }
                }

                describe("when the keychain does not have a valid fxa account") {
                    it("pushes a nil syncCredential") {
                        let syncInfo = try! self.subject.syncCredentials.toBlocking().first()!
                        expect(syncInfo).to(beNil())
                    }
                }
            }

            describe("upgrade") {
                describe("when the upgrade is happening") {
                    beforeEach {
                        self.dispatcher.fakeRegistration.onNext(LifecycleAction.upgrade(from: 1, to: 2))
                    }

                    it("pushes out that the user has an old-style account") {
                        let hasOldAccountInfo = try! self.subject.hasOldAccountInformation.toBlocking().first()!
                        expect(hasOldAccountInfo).to(beTrue())
                    }
                }

                describe("when the keychain does not have old login information") {
                    it("pushes out that the user does not have an old-style account") {
                        let hasOldAccountInfo = try! self.subject.hasOldAccountInformation.toBlocking().first()!
                        expect(hasOldAccountInfo).to(beFalse())
                    }
                }
            }

            // tricky to test because OAuth is hard to fake out :p
            xdescribe("oauthRedirect") {
                var syncCredObserver = self.scheduler.createObserver(SyncCredential?.self)
                var profileObserver = self.scheduler.createObserver(Profile?.self)

                beforeEach {
                    syncCredObserver = self.scheduler.createObserver(SyncCredential?.self)
                    profileObserver = self.scheduler.createObserver(Profile?.self)

                    self.subject.syncCredentials
                            .bind(to: syncCredObserver)
                            .disposed(by: self.disposeBag)

                    self.subject.profile
                            .bind(to: profileObserver)
                            .disposed(by: self.disposeBag)

                    self.dispatcher.fakeRegistration.onNext(AccountAction.oauthRedirect(url: URL(string: "https://lockbox.firefox.com/fxa/ios-redirect.html?code=571e3f5b2d634b844c12e68047be260000927f4babd275638a087b21cc1c40c3&state=wMNWDXCGmjNfru0xfk-iCg&action=signin")!))
                }

                it("saves the accountJSON on successful login") {
                    expect(self.keychainManager.saveArguments[KeychainKey.accountJSON.rawValue]).notTo(beNil())
                }

                it("pushes populated profile and oauthInfo objects to observers") {
                    expect(syncCredObserver.events.first!.value.element!).notTo(beNil())
                    expect(profileObserver.events.first!.value.element!).notTo(beNil())
                }
            }

            describe(".clear") {
                beforeEach {
                    self.dispatcher.fakeRegistration.onNext(AccountAction.clear)
                }

                it("clears all available keychain keys") {
                    for key in KeychainKey.allValues {
                        expect(self.keychainManager.removeArguments).to(contain(key.rawValue))
                    }

                    for key in KeychainKey.allValues {
                        expect(self.keychainManager.removeArguments).to(contain(key.rawValue))
                    }
                }

                it("pushes nil profile and oauth info to observers") {
                    let syncInfo = try! self.subject.syncCredentials.toBlocking().first()!
                    expect(syncInfo).to(beNil())

                    let profile = try! self.subject.profile.toBlocking().first()!
                    expect(profile).to(beNil())
                }

                it("removes all cached URL responses") {
                    expect(self.urlCache.removeCachedResponsesCalled).to(beTrue())
                }

                xit("fetches all available data records and removes them") {
                    // can't subclass WKWebSiteDataStore sufficiently :(
                }
            }

            describe("oauthSignInMessageRead") {
                beforeEach {
                    KeychainWrapper.standard.set("something", forKey: KeychainKey.avatarURL.rawValue)
                    KeychainWrapper.standard.set("is", forKey: KeychainKey.displayName.rawValue)
                    KeychainWrapper.standard.set("in here", forKey: KeychainKey.email.rawValue)
                    self.dispatcher.fakeRegistration.onNext(AccountAction.oauthSignInMessageRead)
                }

                it("clears keychain.standard values associated with old accounts") {
                    for key in KeychainKey.oldAccountValues {
                        expect(KeychainWrapper.standard.hasValue(forKey: key.rawValue)).to(beFalse())
                    }

                    let oldAccountPresent = try! self.subject.hasOldAccountInformation.toBlocking().first()!
                    expect(oldAccountPresent).to(beFalse())
                }
            }
        }
    }
}
