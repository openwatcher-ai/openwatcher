export const INSTALL_GUIDES = [
  {
    id: "xiaomi",
    label: "小米手表",
    models: "Watch S / Watch 2 系列",
    icon: "🟧",
    title: "小米手表教程",
    subtitle: "按图文步骤打开开发者模式、无线调试和配对页面。",
    steps: [
      {
        title: "打开设置",
        body: "在手表主界面进入设置，再找到关于手表或系统信息。",
        visual: "设置首页",
        bullets: ["从表盘进入应用列表。", "找到设置并进入关于手表。"]
      },
      {
        title: "启用开发者模式",
        body: "连续点击系统版本或构建号，直到看到开发者模式已开启。",
        visual: "关于手表",
        bullets: ["通常连续点击 7 次。", "看到提示后返回上一层。"]
      },
      {
        title: "打开无线调试",
        body: "进入开发者选项，先打开调试开关，再打开无线调试。",
        visual: "开发者选项",
        bullets: ["先开启开发者选项。", "确认无线调试已经打开。"]
      },
      {
        title: "打开“使用配对码配对设备”",
        body: "在无线调试页面里进入“使用配对码配对设备”，不要停留在总开关页。",
        visual: "配对页面入口",
        bullets: ["进入真正显示配对信息的页面。", "保持页面不要退出。"]
      },
      {
        title: "记录配对信息",
        body: "记下手表 IP、配对端口、连接端口和配对码，准备回到电脑继续。",
        visual: "配对信息页",
        bullets: ["下一步会用到这里显示的信息。", "建议先核对一遍再返回电脑。"]
      }
    ]
  },
  {
    id: "oppo",
    label: "OPPO 手表",
    models: "Watch 3 / Watch 4 系列",
    icon: "🟩",
    title: "OPPO 手表教程",
    subtitle: "不同机型入口名可能略有差异，整体路径基本一致。",
    steps: [
      {
        title: "打开设置",
        body: "进入设置，找到系统信息或关于设备。",
        visual: "设置入口",
        bullets: ["先进入设置。", "再进入系统信息。"]
      },
      {
        title: "启用开发者模式",
        body: "连续点击版本信息，直到看到开发者模式提示。",
        visual: "系统信息",
        bullets: ["重复点击版本号。", "看到提示后返回。"]
      },
      {
        title: "打开无线调试",
        body: "进入开发者选项，依次打开调试开关和无线调试。",
        visual: "开发者选项",
        bullets: ["先开调试。", "再开无线调试。"]
      },
      {
        title: "打开“使用配对码配对设备”",
        body: "在无线调试页面里进入“使用配对码配对设备”。",
        visual: "配对页面入口",
        bullets: ["进入显示配对信息的页面。", "不要停留在无线调试总开关页。"]
      },
      {
        title: "记录配对信息",
        body: "记下页面上的地址、端口和配对码，保持页面开启。",
        visual: "配对信息页",
        bullets: ["下一步会填写这些信息。", "保持页面不要超时。"]
      }
    ]
  },
  {
    id: "vivo",
    label: "vivo 手表",
    models: "WATCH 2 / WATCH 3 系列",
    icon: "🔵",
    title: "vivo 手表教程",
    subtitle: "先打开开发者模式，再进入无线调试页面。",
    steps: [
      {
        title: "打开设置",
        body: "进入设置，找到关于手表。",
        visual: "设置页",
        bullets: ["先进入设置。", "再找到关于手表。"]
      },
      {
        title: "启用开发者模式",
        body: "连续点击版本信息，直到看到已开启的提示。",
        visual: "版本信息",
        bullets: ["连续点击版本信息。", "看到提示后返回。"]
      },
      {
        title: "打开无线调试",
        body: "进入开发者选项后，打开无线调试。",
        visual: "无线调试",
        bullets: ["先进入开发者选项。", "确认无线调试已打开。"]
      },
      {
        title: "打开“使用配对码配对设备”",
        body: "在无线调试页面里进入配对页面。",
        visual: "配对入口",
        bullets: ["进入显示配对信息的页面。", "准备在电脑里填写信息。"]
      },
      {
        title: "记录配对信息",
        body: "记下手表 IP、两个端口和配对码。",
        visual: "配对信息页",
        bullets: ["记录地址。", "记录配对码。"]
      }
    ]
  },
  {
    id: "samsung",
    label: "Samsung Watch",
    models: "Galaxy Watch 系列",
    icon: "🔷",
    title: "Samsung Watch 教程",
    subtitle: "Galaxy Watch 的菜单名称可能不同，但路径基本一致。",
    steps: [
      {
        title: "打开设置",
        body: "进入设置，再找到关于手表。",
        visual: "设置页",
        bullets: ["进入设置。", "打开关于手表。"]
      },
      {
        title: "启用开发者模式",
        body: "连续点击软件版本信息，直到看到提示。",
        visual: "软件版本",
        bullets: ["重复点击版本信息。", "看到提示后返回。"]
      },
      {
        title: "打开无线调试",
        body: "在开发者选项里开启无线调试。",
        visual: "开发者选项",
        bullets: ["先进入开发者选项。", "确认无线调试已开启。"]
      },
      {
        title: "打开“使用配对码配对设备”",
        body: "进入无线调试里的配对页面。",
        visual: "配对页面入口",
        bullets: ["进入显示配对信息的页面。", "确认页面已保持开启。"]
      },
      {
        title: "记录配对信息",
        body: "记下地址、端口和配对码，回到电脑继续。",
        visual: "配对信息页",
        bullets: ["记录地址。", "保持页面开启。"]
      }
    ]
  },
  {
    id: "pixel",
    label: "Pixel Watch",
    models: "Pixel Watch 系列",
    icon: "🌈",
    title: "Pixel Watch 教程",
    subtitle: "可按系统设置中的开发者路径完成开启。",
    steps: [
      {
        title: "打开设置",
        body: "进入设置，找到系统与关于。",
        visual: "系统设置",
        bullets: ["进入设置。", "找到系统与关于。"]
      },
      {
        title: "启用开发者模式",
        body: "连续点击版本信息，直到看到提示。",
        visual: "版本信息",
        bullets: ["重复点击版本信息。", "看到提示后返回。"]
      },
      {
        title: "打开无线调试",
        body: "进入开发者选项后开启无线调试。",
        visual: "开发者选项",
        bullets: ["先打开开发者选项。", "再打开无线调试。"]
      },
      {
        title: "打开“使用配对码配对设备”",
        body: "进入无线调试中的配对页面。",
        visual: "配对页面入口",
        bullets: ["进入显示配对信息的页面。", "确认页面已打开。"]
      },
      {
        title: "记录配对信息",
        body: "在配对页里记下地址、端口和配对码。",
        visual: "配对信息页",
        bullets: ["记录地址。", "记录配对码。"]
      }
    ]
  }
]
