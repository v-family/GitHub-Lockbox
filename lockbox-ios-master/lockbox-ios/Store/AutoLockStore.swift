/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import Foundation
import RxSwift
import RxCocoa
import Foundation

class AutoLockStore: BaseAutoLockStore {
    static let shared = AutoLockStore()

    private let lifecycleStore: LifecycleStore

    init(dispatcher: Dispatcher = Dispatcher.shared,
         dataStore: DataStore = DataStore.shared,
         lifecycleStore: LifecycleStore = .shared,
         userDefaults: UserDefaults = UserDefaults(suiteName: Constant.app.group) ?? .standard
        ) {
        self.lifecycleStore = lifecycleStore

        super.init(dispatcher: dispatcher,
                   dataStore: dataStore,
                   userDefaults: userDefaults)
    }

    override func initialized() {
        self.dataStore.locked
            .skip(1)
            .subscribe(onNext: { [weak self] locked in
                if locked {
                    self?.stopTimer()
                } else {
                    self?.setupTimer()
                }
            })
            .disposed(by: self.disposeBag)

        self.dispatcher.register
            .filter { action -> Bool in
                // future user interaction actions will need to get added to this list
                action is MainRouteAction ||
                    action is SettingRouteAction ||
                    action is CopyAction ||
                    action is ExternalLinkAction ||
                    action is ItemListDisplayAction ||
                    action is ItemDetailDisplayAction ||
                    action is SettingAction
            }
            .subscribe(onNext: { [weak self] _ in
                self?.resetTimer()
            })
            .disposed(by: self.disposeBag)

        self.dispatcher.register
            .filterByType(class: ExternalWebsiteRouteAction.self)
            .subscribe(onNext: { [weak self] _ in
                self?.pauseTimer()
            })
            .disposed(by: self.disposeBag)

        Observable.combineLatest(self.dispatcher.register, self.dataStore.locked)
            .filter { !$0.1 }
            .map { $0.0 }
            .filterByType(class: LifecycleAction.self)
            .filter { $0 == LifecycleAction.foreground }
            .subscribe(onNext: { [weak self] _ in
                self?.paused = false
                self?.setupTimer()
            })
            .disposed(by: self.disposeBag)
    }
}
