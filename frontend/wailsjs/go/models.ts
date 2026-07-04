export namespace audit {
	
	export class AuditLogger {
	
	
	    static createFrom(source: any = {}) {
	        return new AuditLogger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace capture {
	
	export class AdapterStatusInfo {
	    name: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AdapterStatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class CaptureManager {
	
	
	    static createFrom(source: any = {}) {
	        return new CaptureManager(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace config {
	
	export class RemoteHost {
	    ID: string;
	    Kind: string;
	    Name: string;
	    Host: string;
	    Port: number;
	    Username: string;
	    AuthType: string;
	    PrivateKeyPath: string;
	    LastUsed: string;
	    UseTmux: boolean;
	    TmuxSessionName: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Kind = source["Kind"];
	        this.Name = source["Name"];
	        this.Host = source["Host"];
	        this.Port = source["Port"];
	        this.Username = source["Username"];
	        this.AuthType = source["AuthType"];
	        this.PrivateKeyPath = source["PrivateKeyPath"];
	        this.LastUsed = source["LastUsed"];
	        this.UseTmux = source["UseTmux"];
	        this.TmuxSessionName = source["TmuxSessionName"];
	    }
	}
	export class CustomPattern {
	    Name: string;
	    Regex: string;
	    Action: string;
	
	    static createFrom(source: any = {}) {
	        return new CustomPattern(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Regex = source["Regex"];
	        this.Action = source["Action"];
	    }
	}
	export class AppConfig {
	    CustomPatterns: CustomPattern[];
	    Provider: string;
	    Model: string;
	    CustomPrompt: string;
	    ATSPIPollingMs: number;
	    ClipboardClearSecs: number;
	    HotkeyCopyLast: string;
	    HotkeyFocusWindow: string;
	    Theme: string;
	    FontSize: number;
	    ContextLines: number;
	    OllamaHost: string;
	    LMStudioHost: string;
	    RemoteHosts: RemoteHost[];
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CustomPatterns = this.convertValues(source["CustomPatterns"], CustomPattern);
	        this.Provider = source["Provider"];
	        this.Model = source["Model"];
	        this.CustomPrompt = source["CustomPrompt"];
	        this.ATSPIPollingMs = source["ATSPIPollingMs"];
	        this.ClipboardClearSecs = source["ClipboardClearSecs"];
	        this.HotkeyCopyLast = source["HotkeyCopyLast"];
	        this.HotkeyFocusWindow = source["HotkeyFocusWindow"];
	        this.Theme = source["Theme"];
	        this.FontSize = source["FontSize"];
	        this.ContextLines = source["ContextLines"];
	        this.OllamaHost = source["OllamaHost"];
	        this.LMStudioHost = source["LMStudioHost"];
	        this.RemoteHosts = this.convertValues(source["RemoteHosts"], RemoteHost);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace keychain {
	
	export class Client {
	
	
	    static createFrom(source: any = {}) {
	        return new Client(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace memguard {
	
	export class Enclave {
	
	
	    static createFrom(source: any = {}) {
	        return new Enclave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace services {
	
	export class ExportMessage {
	    role: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class LLMService {
	
	
	    static createFrom(source: any = {}) {
	        return new LLMService(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class RemoteConnectParams {
	    kind: string;
	    host: string;
	    port: number;
	    username: string;
	    authType: string;
	    password?: string;
	    privateKeyPath?: string;
	    passphrase?: string;
	    savePassword: boolean;
	    savedHostId?: string;
	    useTmux: boolean;
	    tmuxSessionName?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteConnectParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.password = source["password"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.passphrase = source["passphrase"];
	        this.savePassword = source["savePassword"];
	        this.savedHostId = source["savedHostId"];
	        this.useTmux = source["useTmux"];
	        this.tmuxSessionName = source["tmuxSessionName"];
	    }
	}

}

