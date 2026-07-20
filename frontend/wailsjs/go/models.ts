export namespace config {
	
	export class Config {
	    theme: string;
	    opacity: number;
	    autoLockEnabled: boolean;
	    autoLockMinutes: number;
	    lastVault: string;
	    recentVaults: string[];
	    generatorLength: number;
	    generatorSymbols: boolean;
	    toastSeconds: number;
	    apiAutoStart: boolean;
	    apiPort: number;
	    apiKey: string;
	    apiAllowlist: string[];
	    apiHttps: boolean;
	    updateAutoCheck: boolean;
	    updateSkippedVersion: string;
	    updateLaterUntil: string;
	    updateLastAutoCheck: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.opacity = source["opacity"];
	        this.autoLockEnabled = source["autoLockEnabled"];
	        this.autoLockMinutes = source["autoLockMinutes"];
	        this.lastVault = source["lastVault"];
	        this.recentVaults = source["recentVaults"];
	        this.generatorLength = source["generatorLength"];
	        this.generatorSymbols = source["generatorSymbols"];
	        this.toastSeconds = source["toastSeconds"];
	        this.apiAutoStart = source["apiAutoStart"];
	        this.apiPort = source["apiPort"];
	        this.apiKey = source["apiKey"];
	        this.apiAllowlist = source["apiAllowlist"];
	        this.apiHttps = source["apiHttps"];
	        this.updateAutoCheck = source["updateAutoCheck"];
	        this.updateSkippedVersion = source["updateSkippedVersion"];
	        this.updateLaterUntil = source["updateLaterUntil"];
	        this.updateLastAutoCheck = source["updateLastAutoCheck"];
	    }
	}

}

export namespace main {
	
	export class APIStatus {
	    running: boolean;
	    port: number;
	    url: string;
	    tls: boolean;
	    fingerprint: string;
	
	    static createFrom(source: any = {}) {
	        return new APIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.tls = source["tls"];
	        this.fingerprint = source["fingerprint"];
	    }
	}
	export class UpdateInfo {
	    checking: boolean;
	    installing: boolean;
	    progress: string;
	    available: boolean;
	    version: string;
	    notes: string;
	    current: string;
	    checkedAt: string;
	    error: string;
	    notify: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checking = source["checking"];
	        this.installing = source["installing"];
	        this.progress = source["progress"];
	        this.available = source["available"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.current = source["current"];
	        this.checkedAt = source["checkedAt"];
	        this.error = source["error"];
	        this.notify = source["notify"];
	    }
	}

}

export namespace vault {
	
	export class AuditEntry {
	    ts: string;
	    actor: string;
	    action: string;
	    secret_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ts = source["ts"];
	        this.actor = source["actor"];
	        this.action = source["action"];
	        this.secret_id = source["secret_id"];
	    }
	}
	export class Category {
	    id: string;
	    name: string;
	    color?: string;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	    }
	}
	export class Secret {
	    id: string;
	    title: string;
	    username?: string;
	    password?: string;
	    url?: string;
	    notes?: string;
	    category_id?: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Secret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.category_id = source["category_id"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class SecretInput {
	    Title: string;
	    Username: string;
	    Password: string;
	    URL: string;
	    Notes: string;
	    CategoryID: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Title = source["Title"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	        this.URL = source["URL"];
	        this.Notes = source["Notes"];
	        this.CategoryID = source["CategoryID"];
	    }
	}

}

