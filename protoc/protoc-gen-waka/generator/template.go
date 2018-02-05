package generator

const CSharpHandlerTemplate = `// Generated by github.com/liuhan907/waka/protoc/protoc-gen-waka
// DO NOT EDIT!!!

using CellnetSDK;
using CellnetSDK.Meta;
using Google.Protobuf;
using System;
using System.Linq;

namespace WakaSDK
{
    /// <summary>
    /// 处理器
    /// </summary>
    public partial class Supervisor
    {
        {{range .RPC}}
        /// <summary>
{{.LeadingComments}}
        /// </summary>
        static public void Request{{.Name}}({{$.Namespace}}.{{.InputType}} request, Action< string, {{$.Namespace}}.{{.OutputType}} > andThen)
        {
            var req = BuildFutureRequest(request);
            ThenTable.Add(req.Number, (status, x) =>
            {
                var ev = ({{$.Namespace}}.{{.OutputType}})x;
                andThen(status, ev);
            });
            Session?.Send(req);
        }
        {{end}}

        {{range .Post}}
        /// <summary>
{{.LeadingComments}}
        /// </summary>
        static public void Post{{.Type}}({{$.Namespace}}.{{.Type}} request)
        {
            var req = BuildTransportRequest(request);
            Session?.Send(req);
        }
        {{end}}

        static private void DispatchTransport(ISession ses, uint id, IMessage msg, byte[] rawData)
        {
            var transport = (WakaProto.Transport)msg;
            if (!MetaTable.TryGetMessageMetaByID(transport.Id, out MessageMeta meta))
            {
                throw new ArgumentException("unknown message");
            }
            var message = meta.ParseFrom(transport.Payload.ToArray());
            switch (meta.Name)
            {
                {{range .Receive}}
                case "{{$.Namespace}}.{{.Type}}":
                    Dispatcher?.Event{{.Type}}(({{$.Namespace}}.{{.Type}})message);
                    break;
                {{end}}
                default:
                    break;
            }
        }
    }
}

`

const CSharpIDispatcherTemplate = `// Generated by github.com/liuhan907/waka/protoc/protoc-gen-waka
// DO NOT EDIT!!!

namespace WakaSDK
{
    /// <summary>
    /// 消息分发器接口
    /// </summary>
    public partial interface IDispatcher
    {
        /// <summary>
        /// 连接建立
        /// </summary>
        void Connected();
    
        /// <summary>
        /// 连接失败
        /// </summary>
        void ConnectFailed();
    
        /// <summary>
        /// 连接断开
        /// </summary>
        void Closed();

        {{range .Receive}}
        /// <summary>
{{.LeadingComments}}
        /// </summary>
        void Event{{.Type}}({{$.Namespace}}.{{.Type}} ev);
        {{end}}
    }
}

`

const CSharpSupervisorTemplate = `// Generated by github.com/liuhan907/waka/protoc/protoc-gen-waka
// DO NOT EDIT!!!

using CellnetSDK;
using CellnetSDK.Meta;
using CellnetSDK.Socket;
using Google.Protobuf;
using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;

namespace WakaSDK
{
    /// <summary>
    /// SDK接口
    /// </summary>
    public partial class Supervisor
    {
        static private EventQueue Evq = null;
        static private Callback Callback = null;
        static private Connector Connector = null;
        static private ISession Session = null;
        static private IDispatcher Dispatcher = null;
        static private Dictionary<ulong, Action<string, object>> ThenTable = null;
        static private long Number = 1;
        static private DateTime LastRemoteHeartTime = DateTime.UtcNow;
        static private DateTime LastLocalHeartTime = DateTime.UtcNow;

        /// <summary>
        /// 设置推送消息处理器
        /// </summary>
        /// <param name="dispatcher">用户定义的处理器对象</param>
        static public void SetDispatcher(IDispatcher dispatcher)
        {
            Dispatcher = dispatcher;
        }

        /// <summary>
        /// 连接服务器
        /// </summary>
        /// <param name="host"></param>
        /// <param name="port"></param>
        static public void Connect(string host, int port)
        {
            Evq.Clear();
            ThenTable.Clear();
            Connector.ConnectAsync(host, port);
        }

        /// <summary>
        /// 关闭连接
        /// </summary>
        static public void Close()
        {
            Session?.Close();
            Session = null;
        }

        /// <summary>
        /// 处理委托队列
        /// </summary>
        static public void Loop()
        {
            if ((long)((DateTime.UtcNow - LastRemoteHeartTime).TotalSeconds) >= 30)
            {
                Close();
            }
            if ((long)((DateTime.UtcNow - LastLocalHeartTime).TotalSeconds) >= 3)
            {
                Session?.Send(new WakaProto.Heart());
                LastLocalHeartTime = DateTime.UtcNow;
            }
            Evq.Loop();
        }

        static private void Connected(ISession s)
        {
            LastRemoteHeartTime = DateTime.UtcNow;
            Session = s;
            Dispatcher?.Connected();
        }

        static private void ConnectFailed()
        {
            Session = null;
            Dispatcher?.ConnectFailed();
        }

        static private void Closed(ISession s)
        {
            Session = null;
            Dispatcher?.Closed();
        }

        static private void RedirectFutureResponse(ISession ses, uint id, IMessage message, byte[] rawData)
        {
            var response = (WakaProto.FutureResponse)message;
            if (ThenTable.TryGetValue(response.Number, out Action<string, object> andThen))
            {
                ThenTable.Remove(response.Number);

                if (response.Status != "success")
                {
                    andThen(response.Status, null);
                }
                else if (MetaTable.TryGetMessageMetaByID(response.Id, out MessageMeta meta))
                {
                    andThen(response.Status, meta.ParseFrom(response.Payload.ToArray()));
                }
                else
                {
                    andThen("failed: unknown response type", null);
                }
            }
        }

        static private void RedirectTransport(ISession ses, uint id, IMessage message, byte[] rawData)
        {
            DispatchTransport(ses, id, message, rawData);
        }

        static private void RedirectHeart(ISession ses, uint id, IMessage message, byte[] rawData)
        {
            LastRemoteHeartTime = DateTime.UtcNow;
        }

        static private WakaProto.FutureRequest BuildFutureRequest(IMessage request)
        {
            MessageMeta meta;
            if (!MetaTable.TryGetMessageMetaByType(request.GetType(), out meta))
            {
                throw new ArgumentException("unknown message");
            }
            return new WakaProto.FutureRequest
            {
                Id = meta.ID,
                Payload = request.ToByteString(),
                Number = (ulong)Interlocked.Increment(ref Number),
            };
        }

        static private WakaProto.Transport BuildTransportRequest(IMessage request)
        {
            MessageMeta meta;
            if (!MetaTable.TryGetMessageMetaByType(request.GetType(), out meta))
            {
                throw new ArgumentException("unknown message");
            }
            return new WakaProto.Transport
            {
                Id = meta.ID,
                Payload = request.ToByteString(),
            };
        }

        static Supervisor()
        {
            WakaProto.WakaProtoMetaProvider.RegisterAll();
            {{.Namespace}}.{{.MetaProviderClassName}}.RegisterAll();

            Evq = new EventQueue();
            Callback = new Callback()
                .RegisterConnectFailed(ConnectFailed)
                .RegisterConnected(Connected)
                .RegisterClosed(Closed)
                .RegisterMessage(new WakaProto.FutureResponse().GetType(), RedirectFutureResponse)
                .RegisterMessage(new WakaProto.Transport().GetType(), RedirectTransport)
                .RegisterMessage(new WakaProto.Heart().GetType(), RedirectHeart);
            Connector = new Connector(Evq, Callback);

            ThenTable = new Dictionary<ulong, Action<string, object>>();
        }
    }
}

`