# 开发注意事项

- Windows 文件名不区分大小写，组件交互检查不得仅用首字母大小写与 SQL 逻辑检查区分。例如使用 TableDesignerComponent.test.ts 与 tableDesigner.test.ts，避免覆盖已有文件。

- 可拖动预览窗口使用 transform，会改变子元素 position:fixed 的包含块。共享下拉不能直接把视口坐标写入该包含块，必须扣除变换祖先的原点；新增弹窗应同时检查键盘选择和可见列表位置，不能只验证输入回填。

- 主窗口关闭不能在 WindowClosing 通知中直接 Quit：那时草稿还没获得确认。使用可取消钩子先阻止关闭，再异步通知前端，批准后的退出只执行一次；不能在关闭钩子里同步等待 WebView。
