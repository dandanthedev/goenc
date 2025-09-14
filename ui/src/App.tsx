import { useEffect, useState } from "react";

async function customFetch(
  path: string,
  options: RequestInit,
  overrideKey?: string
) {
  if (import.meta.env.VITE_API_URL) {
    path = import.meta.env.VITE_API_URL + path;
  }
  const apiKey = overrideKey || localStorage.getItem("apiKey") || "";
  options.headers = {
    ...options.headers,
    "x-api-key": apiKey,
  };
  const response = await fetch(path, options);
  if (response.status === 204) {
    return;
  }
  const json = await response.json();
  if (json.error) throw new Error(json.error);
  if (!json.success) throw new Error("Malformed response from server");
  return json.data || json;
}

function App() {
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [checkingKey, setCheckingKey] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);

  useEffect(() => {
    const apiKey = localStorage.getItem("apiKey");
    if (apiKey) {
      setApiKey(apiKey);
    }
  }, []);

  function resetAuth() {
    setApiKey(null);
    setCheckingKey(false);
    setAuthError(null);
  }

  if (!apiKey) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-blue-50 to-gray-100">
        <div className="bg-white shadow-lg rounded-xl p-8 w-full max-w-sm flex flex-col items-center">
          <div className="text-3xl font-extrabold text-blue-700 mb-2 tracking-tight">
            GoEnc
          </div>
          <div className="text-gray-500 mb-4">Sign in with your API Key</div>
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              setCheckingKey(true);
              const key = (e.target as HTMLFormElement).apiKey.value;
              try {
                await customFetch(
                  "/api/auth",
                  {
                    method: "GET",
                  },
                  key
                );
                localStorage.setItem("apiKey", key);
                setApiKey(key);
              } catch (e) {
                const message = e instanceof Error ? e.message : String(e);
                setAuthError(message);
              } finally {
                setCheckingKey(false);
              }
            }}
            className="w-full"
          >
            <input
              type="text"
              placeholder="API Key"
              className="mt-2 p-3 rounded-lg border-2 border-blue-200 focus:border-blue-500 w-full outline-none transition"
              id="apiKey"
              disabled={checkingKey}
              required
            />
            <button
              className="mt-4 p-3 rounded-lg bg-blue-600 text-white font-semibold hover:bg-blue-700 w-full disabled:opacity-50 transition"
              disabled={checkingKey}
              type="submit"
            >
              {checkingKey ? "Checking..." : "Sign in"}
            </button>
            {authError && (
              <div className="mt-2 text-red-500 text-center">{authError}</div>
            )}
          </form>
        </div>
      </div>
    );
  }

  return <SignedIn resetAuth={resetAuth} />;
}

function SignedIn({ resetAuth }: { resetAuth: () => void }) {
  const [videos, setVideos] = useState<
    {
      id: string;
      sizes: string[];
    }[]
  >([]);

  const [queue, setQueue] = useState<
    {
      id: string;
      source: string;
      profiles: string;
      status: string;
      step: string;
      attempts: number;
    }[]
  >([]);

  const [profiles, setProfiles] = useState<string[]>([]);

  const [uploading, setUploading] = useState(false);
  const [uploadPercent, setUploadPercent] = useState(0);

  useEffect(() => {
    function fetchData() {
      fetchVideos();
      fetchQueue();
      fetchProfiles();
    }
    fetchData();
  }, []);

  useEffect(() => {
    //refresh queue every 10 seconds
    const interval = setInterval(() => {
      fetchQueue();
    }, 10000);
    return () => clearInterval(interval);
  }, []);

  async function fetchVideos() {
    const data = await customFetch("/api/videos/list", { method: "GET" });
    const videoIds = data.videos;
    //simultaniously fetch all video info
    const videoInfoPromises = videoIds.map(async (id: string) => {
      const data = await customFetch(`/api/videos/${id}`, { method: "GET" });
      return data;
    });
    const videoInfo = await Promise.all(videoInfoPromises);
    setVideos(videoInfo);
  }

  async function removeVideo(id: string) {
    await customFetch(`/api/videos/${id}`, { method: "DELETE" });
    await fetchVideos();
  }

  async function watchVideo(id: string) {
    try {
      const token = await customFetch("/api/token", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          id: id,
          expires: "1h",
          attributes: { controls: true },
        }),
      });
      window.open(`${import.meta.env.VITE_API_URL ?? ""}${token.playerUrl}`);
    } catch (error) {
      console.error("Error generating token:", error);
      alert("Failed to generate token");
    }
  }

  async function fetchQueue() {
    const data = await customFetch("/api/queue", { method: "GET" });
    setQueue(data);
  }

  async function restoreStuckJobs() {
    await customFetch("/api/queue/recover", { method: "POST" });
    await fetchQueue();
  }

  async function cleanupJobs() {
    await customFetch("/api/queue/cleanup", { method: "POST" });
    await fetchQueue();
  }

  function removeToken() {
    localStorage.removeItem("apiKey");
    resetAuth();
  }

  async function fetchProfiles() {
    const data = await customFetch("/api/profiles", { method: "GET" });
    setProfiles(data);
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-blue-50 to-gray-100">
      <div className="bg-white shadow-xl rounded-2xl p-8 w-full flex flex-col items-center">
        <div className="text-3xl font-extrabold text-blue-700 mb-2 tracking-tight">
          GoEnc
        </div>
        <button
          className="p-3 rounded-lg bg-red-500 text-white font-semibold hover:bg-red-700 transition mb-4"
          onClick={() => removeToken()}
        >
          Sign out
        </button>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setUploading(true);
            setUploadPercent(0);
            const formData = new FormData(e.target as HTMLFormElement);
            const id = formData.get("id") as string;
            const file = formData.get("file") as File;
            const profiles = formData.get("profiles") as string;

            const queryParams = new URLSearchParams();
            queryParams.append("id", id);
            queryParams.append("profiles", profiles);

            const req = new XMLHttpRequest();
            req.open(
              "POST",
              `${import.meta.env.VITE_API_URL ?? ""}/api/upload?${queryParams}`
            );
            req.upload.onprogress = (e) => {
              if (e.lengthComputable) {
                setUploadPercent(Math.round((e.loaded / e.total) * 100));
              }
            };
            req.onload = () => {
              const data = JSON.parse(req.responseText);
              if (req.status === 200) {
                if (data.id === id) {
                  setUploading(false);
                  fetchQueue();
                } else {
                  setUploading(false);
                  alert("Failed to upload file");
                }
              } else {
                setUploading(false);
                alert(data.error || "Failed to upload file");
              }
            };
            req.setRequestHeader(
              "x-api-key",
              localStorage.getItem("apiKey") as string
            );

            //make formdata with file
            const reqFormData = new FormData();
            reqFormData.append("file", file);

            //send
            req.send(reqFormData);
          }}
        >
          <input
            type="text"
            placeholder="ID"
            className="mt-2 p-3 rounded-lg border-2 border-blue-200 focus:border-blue-500 w-full outline-none transition"
            required
            disabled={uploading}
            name="id"
          />
          <input
            type="file"
            className="mt-2 p-3 rounded-lg border-2 border-blue-200 focus:border-blue-500 w-full outline-none transition"
            required
            disabled={uploading}
            name="file"
          />
          <select
            multiple
            className="mt-2 p-3 rounded-lg border-2 border-blue-200 focus:border-blue-500 w-full outline-none transition"
            required
            disabled={uploading}
            name="profiles"
          >
            {profiles.map((profile) => (
              <option key={profile}>{profile}</option>
            ))}
          </select>
          <button
            className="mt-4 p-3 rounded-lg bg-blue-600 text-white font-semibold hover:bg-blue-700 w-full disabled:opacity-50 transition mb-3"
            disabled={uploading}
          >
            Upload {uploading && `(${uploadPercent}%)`}
          </button>
        </form>
        <div className="w-full flex flex-col lg:flex-row gap-8">
          <div className="flex-1">
            <div className="text-xl font-bold mb-2 text-blue-800">Videos</div>
            <div className="flex gap-2 items-center justify-center mb-4">
              <button
                className="p-2 rounded-md bg-blue-500 text-white hover:bg-blue-700 font-semibold transition"
                onClick={fetchVideos}
              >
                refresh
              </button>
            </div>
            <div className="space-y-4">
              {videos.map((video) => (
                <div
                  key={video.id}
                  className="bg-blue-50 border border-blue-100 rounded-lg p-4 flex flex-col md:flex-row md:items-center md:justify-between shadow-sm hover:shadow-md transition"
                >
                  <div className="font-mono text-blue-900 text-lg">
                    <span className="font-semibold">{video.id}</span>
                    <span className="ml-2 text-sm text-blue-600">
                      [{video.sizes.join(", ")}]
                    </span>
                  </div>
                  <div className="mt-2 md:mt-0 flex gap-2">
                    <button
                      className="p-2 rounded-md bg-blue-500 text-white hover:bg-blue-700 font-semibold transition"
                      onClick={() => watchVideo(video.id)}
                    >
                      Watch
                    </button>
                    <button
                      className="p-2 rounded-md bg-red-500 text-white hover:bg-red-700 font-semibold transition"
                      onClick={() => {
                        if (
                          !confirm(
                            "Are you sure you want to remove this video?"
                          )
                        )
                          return;
                        removeVideo(video.id);
                      }}
                    >
                      Remove
                    </button>
                  </div>
                </div>
              ))}
              {videos.length === 0 && (
                <div className="text-gray-400 text-center">
                  No videos found.
                </div>
              )}
            </div>
          </div>
          <div className="flex-1">
            <div className="text-xl font-bold mb-2 text-blue-800">Queue</div>
            <div className="flex gap-2 items-center justify-center mb-4">
              <button
                className="p-2 rounded-md bg-blue-500 text-white hover:bg-blue-700 font-semibold transition"
                onClick={fetchQueue}
              >
                refresh
              </button>
              <button
                className="p-2 rounded-md bg-red-500 text-white hover:bg-red-700 font-semibold transition"
                onClick={restoreStuckJobs}
              >
                restore stuck jobs
              </button>
              <button
                className="p-2 rounded-md bg-green-500 text-white hover:bg-green-700 font-semibold transition"
                onClick={cleanupJobs}
              >
                cleanup completed jobs
              </button>
            </div>
            <div className="space-y-4">
              {queue.map((video) => (
                <div
                  key={video.id}
                  className="bg-yellow-50 border border-yellow-100 rounded-lg p-4 flex flex-col md:flex-row md:items-center md:justify-between shadow-sm hover:shadow-md transition"
                >
                  <div className="font-mono text-yellow-900 text-lg">
                    <span className="font-semibold">{video.id}</span>
                    <span className="ml-2 text-sm text-yellow-700">
                      {video.source}
                    </span>
                  </div>
                  <div className="mt-2 md:mt-0 text-sm text-yellow-800">
                    <span className="font-semibold">{video.status}</span> (
                    {video.step}, {video.attempts} attempts)
                  </div>
                </div>
              ))}
              {queue.length === 0 && (
                <div className="text-gray-400 text-center">Queue is empty.</div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
