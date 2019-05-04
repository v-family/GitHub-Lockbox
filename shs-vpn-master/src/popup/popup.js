const isProd = false;

if(isProd){
  console.log = () => {};
}


const $ = sel => document.querySelector(sel);

const connected = $("#connected");
const notConnected = $("#not-connected");
const reload = $("button");
const proxies = $("select");
const info = $("p");
const checkbox = $("input");
const section = $("section");

const calc = () => {
  info.innerText = "Loading...";
  chrome.proxy.settings.get({}, settings => {
    if(settings.levelOfControl === "controlled_by_this_extension" && settings.value.mode === "pac_script"){
      connected.hidden = false;
      notConnected.hidden = true;
    }else{
      connected.hidden = false;
      notConnected.hidden = true;
    }
    fetch("http://ip-api.com/json").then(x=>x.json()).then(data => {
      info.innerText = `Your IPv4 is ${data.query}, you're in ${data.city}, ${data.regionName}, ${data.country} and your ISP is ${data.isp}.`;
    }).catch(console.error);
  });
};

const setProxy = () => {
  const proxy = proxies.options[proxies.selectedIndex].value || proxies.children[0].value;
  const disabled = !checkbox.checked;
  console.log("set proxy to", proxy);
  var config = {
    mode: "pac_script",
    pacScript: {
      data: String.raw`function FindProxyForURL(url, host) {
        return '` + (disabled ? "SYSTEM" : proxy) + String.raw`';
      }`
    }
  };
  chrome.proxy.settings.set({value: config, scope: 'regular'}, calc);
};

reload.onclick = setProxy;

proxies.onchange = () => {
  chrome.storage.sync.set({proxy: proxies.value}, setProxy);
};

reload.onclick();

checkbox.onchange = () => {
  section.hidden = !checkbox.checked;
  console.log("set disabled to", !checkbox.checked);
  chrome.storage.sync.set({disabled: !checkbox.checked}, setProxy);
};

chrome.storage.sync.get(["proxy", "disabled"], data => {
  console.log("got data from store:", data, "set checked to", !data.disabled);
  checkbox.checked = !data.disabled;
  let proxy;
  proxy = data.proxy || proxies.children[0].value;
  proxies.value = proxy;
  section.hidden = !checkbox.checked;
  setProxy();
});
