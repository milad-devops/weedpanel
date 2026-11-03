export type Cluster = {
  id: string; name: string; description: string; connection_mode: 'direct' | 'agent'
  master_urls: string[]; filer_url: string; s3_url: string; prometheus_url: string
  status: 'unknown' | 'healthy' | 'degraded' | 'unreachable'; created_at: string
}
const API='/api/v1'
async function request(path:string,init?:RequestInit){
  const response=await fetch(`${API}${path}`,{...init,headers:{...(init?.body instanceof FormData?{}:{'Content-Type':'application/json'}),...(init?.headers??{})},credentials:'same-origin'})
  const body=await response.json().catch(()=>({}))
  if(!response.ok) throw new Error(body?.error?.message||body?.error?.code||`Request failed (${response.status})`)
  return body
}
export async function getSystemInfo(){return request('/system/info')}
export async function getClusters():Promise<Cluster[]>{const b=await request('/clusters');return b.data??[]}
export async function getMe(){return request('/auth/me')}
export async function login(email:string,password:string){return request('/auth/login',{method:'POST',body:JSON.stringify({email,password})})}
export async function logout(){return request('/auth/logout',{method:'POST'})}
export async function createCluster(payload:Partial<Cluster>){return request('/clusters',{method:'POST',body:JSON.stringify(payload)})}
export async function updateCluster(id:string,payload:Partial<Cluster>){return request(`/clusters/${id}`,{method:'PATCH',body:JSON.stringify(payload)})}
export async function deleteCluster(id:string){return request(`/clusters/${id}`,{method:'DELETE'})}
export async function testCluster(id:string){return request(`/clusters/${id}/connection-test`,{method:'POST'})}
export async function getLive(id:string,section:string){const r=await fetch(`${API}/clusters/${id}/live?section=${encodeURIComponent(section)}`,{credentials:'same-origin'});const t=await r.text();if(!r.ok)throw new Error(parseError(t,r.status));try{return JSON.parse(t)}catch{return t}}
function parseError(t:string,status:number){try{return JSON.parse(t)?.error?.message||JSON.parse(t)?.error?.code||`Request failed (${status})`}catch{return `Request failed (${status})`}}
export async function listFiles(id:string,path:string){return request(`/clusters/${id}/files?path=${encodeURIComponent(path)}`)}
export async function uploadFile(id:string,path:string,file:File){const r=await fetch(`${API}/clusters/${id}/files?path=${encodeURIComponent(path)}&name=${encodeURIComponent(file.name)}`,{method:'POST',body:file,headers:{'Content-Type':file.type||'application/octet-stream'},credentials:'same-origin'});if(!r.ok)throw new Error(await r.text()||`Upload failed (${r.status})`);return r.text()}
export async function deleteFile(id:string,path:string){return request(`/clusters/${id}/files?path=${encodeURIComponent(path)}`,{method:'DELETE'})}
export async function getUsers(){return request('/admin/users')}

export async function downloadFile(id:string,path:string):Promise<Blob>{const r=await fetch(`${API}/clusters/${id}/file?path=${encodeURIComponent(path)}`,{credentials:'same-origin'});if(!r.ok)throw new Error(await r.text()||`Download failed (${r.status})`);return r.blob()}

export async function createDirectory(id:string,path:string,name:string){return request(`/clusters/${id}/directories?path=${encodeURIComponent(path)}&name=${encodeURIComponent(name)}`,{method:'POST'})}

export type MetricPoint={timestamp:number;value:string}
export type MetricSeries={metric:Record<string,string>;values:MetricPoint[]}
export type MetricResult={key:string;title:string;unit:string;series:MetricSeries[]}
export type MonitoringData={range:string;start:string;end:string;step:string;metrics:MetricResult[]}
export async function getMetrics(id:string,range:string):Promise<MonitoringData>{const r=await request(`/clusters/${id}/metrics?range=${encodeURIComponent(range)}`);return r.data}
