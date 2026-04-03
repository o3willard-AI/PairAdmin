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

}

export namespace config {
	
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

}

