const isProd = false;
const version_major = 0;
const version_minor = 8;

if(isProd){
  console.log = () => {};
}

chrome.storage.sync.get(["proxy", "disabled", "version_major", "version_minor"], data => {
  if(!("version_major" in data && "version_minor" in data) || data.version_major < version_major || (data.version_major === version_major && data.version_minor < version_minor)){
    console.log("old version");
    data.proxy = "HTTPS shrunkunseeingbacklight.info:443";
    chrome.storage.sync.clear(()=>chrome.storage.sync.set({version_major, version_minor, disabled: false, proxy: "HTTPS shrunkunseeingbacklight.info:443"}));
  }

  let proxy;
  if(!data.disabled){
    proxy = data.proxy || "HTTPS shrunkunseeingbacklight.info:443";
  }
  var config = {
    mode: "pac_script",
    pacScript: {
      data: String.raw`function FindProxyForURL(url, host) {
        return '` + (data.disabled ? "DIRECT" : proxy) + String.raw`';
      }`
    }
  };
  chrome.proxy.settings.set({value: config, scope: 'regular'}, ( )=>{});
});
