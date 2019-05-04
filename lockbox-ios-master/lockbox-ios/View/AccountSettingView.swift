/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

import UIKit
import RxCocoa
import RxSwift

class AccountSettingView: UIViewController {
    @IBOutlet weak var avatarImageView: UIImageView!
    @IBOutlet weak var usernameLabel: UILabel!
    @IBOutlet weak var unlinkAccountButton: UIButton!

    internal var presenter: AccountSettingPresenter?
    private let disposeBag = DisposeBag()

    override var preferredStatusBarStyle: UIStatusBarStyle {
        return UIStatusBarStyle.lightContent
    }

    required init?(coder aDecoder: NSCoder) {
        super.init(coder: aDecoder)
        self.presenter = AccountSettingPresenter(view: self)
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = Constant.color.viewBackground
        self.setupUnlinkAccountButton()
        self.setupNavBar()
        self.presenter?.onViewReady()
    }
}

extension AccountSettingView: AccountSettingViewProtocol {
    func bind(avatarImage: Driver<Data>) {
        avatarImage
                .map { data -> UIImage? in
                    return UIImage(data: data)?.circleCrop(borderColor: Constant.color.cellBorderGrey)
                }
                .filterNil()
                .drive(self.avatarImageView.rx.image)
                .disposed(by: self.disposeBag)
    }

    func bind(displayName: Driver<String>) {
        displayName
                .drive(self.usernameLabel.rx.text)
                .disposed(by: self.disposeBag)
    }

    var unLinkAccountButtonPressed: ControlEvent<Void> {
        return self.unlinkAccountButton.rx.tap
    }

    var onSettingsButtonPressed: ControlEvent<Void>? {
        if let button = self.navigationItem.leftBarButtonItem?.customView as? UIButton {
            return button.rx.tap
        }

        return nil
    }

}

extension AccountSettingView: UIGestureRecognizerDelegate {
    fileprivate func setupUnlinkAccountButton() {
        self.unlinkAccountButton.setBorder(color: Constant.color.cellBorderGrey, width: 0.5)
    }

    fileprivate func setupNavBar() {
        self.navigationItem.title = Constant.string.account
        self.navigationController?.navigationBar.titleTextAttributes = [
            .foregroundColor: UIColor.white,
            .font: UIFont.navigationTitleFont
        ]
        self.navigationController?.navigationBar.accessibilityIdentifier = "accountSetting.navigationBar"
        let leftButton = UIButton(title: Constant.string.settingsTitle, imageName: "back")
        leftButton.titleLabel?.font = .navigationButtonFont
        self.navigationItem.leftBarButtonItem = UIBarButtonItem(customView: leftButton)

        if let presenter = self.presenter {
            self.navigationController?.interactivePopGestureRecognizer?.delegate = self
            self.navigationController?.interactivePopGestureRecognizer?.rx.event
                .map { _ -> Void in
                    return ()
                }
                .bind(to: presenter.onSettingsTap)
                .disposed(by: self.disposeBag)
        }
    }
}
