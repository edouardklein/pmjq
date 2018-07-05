(ns interf.core
  (:require-macros [cljs.core.async.macros :refer [go go-loop]])
  (:require
   [reagent.core :as reagent]
   [cljs.core.async :as async :refer [<! chan put! ]]
   [clojure.string :refer [starts-with?]]
   [goog.crypt.base64 :as b64]
   [interf.page :refer [page]]
   )
)

(defonce result
  ;; List of downloadable URLs
  (reagent/atom []))

(defonce upload
  ;; Dict mapping input dirs to a dict:
  ;; :state nil or "proc" or done
  ;; :file-name 
  (reagent/atom {}))

(defonce logs
  ;; Relevant logs from PMJQ
  (reagent/atom {:show nil  ;; For log toggling feature
                 :text ""}))

(defonce errors
  ;; Dict mapping dirname (for now) to dict:
  ;; :show
  ;; :command
  ;; :message
  (reagent/atom {}))

(defonce queues
  ;; Dict mapping input dirs to the number of files in it
  (reagent/atom {}))


(defn type-message
  "Messages are typed by their :-delimited prefix"
  [event]
  (get (clojure.string/split event.data #":") 0))

(defmulti receive! type-message)

(defmethod receive! :default [event]
  (.log js/console (str  "Unknown: " event.data)))

(defmethod receive! "log" [event]
  ;; Add the log line to the corresponding atom
  (let [text (:text @logs)
        datatext (get (clojure.string/split event.data #":" 2) 1)]
    (.log js/console datatext)
    (swap! logs assoc :text
            (str text
                 (if (seq text) "\n")
                 datatext))))

(defmethod receive! "stderr" [event]
  ;; Receive an error message and add it to the corresponding atom
  (.log js/console "error detected" event.data)
  (let [[proto command message] (clojure.string/split event.data #":" 3)
        mess (b64/decodeString message)]
    (swap! errors assoc command {:command command :message mess})
    (.log js/console mess)))

(defmethod receive! "input" [event]
  ;; Receive an input directive and add it to the corresponding atom
  (.log js/console "got input directive" event.data)
  (swap! upload assoc (-> event.data
                          (clojure.string/split #":" 2)
                          (get 1))

         {:state nil}))

(defmethod receive! "waiting" [event]
  ;; Receive the queue length on a folder and add it to the corresponding atom
  (let [[proto folder num] (clojure.string/split event.data #":" 3)
        previous (get @queues folder)]
      (swap! queues assoc folder num)))

(defmethod receive! "output" [event]
  ;; Receive a hyperlink targeting an output file, and add it to the corresponding atom
  (.log js/console "got output data")
  (swap! result conj (get (clojure.string/split event.data #":" 2) 1)))

(defmethod receive! "done" [event]
  ;; Receive message signaling end and success (not used yet)
  (.log js/console "All done"))

(defn init-websocket
  "Initiate a connection to the websocket server"
  []
  (.log js/console "Websocket")
  (let [websoc (atom (js/WebSocket. (str "ws://" js/window.location.hostname)))]
    (aset @websoc "onmessage" (fn [x] (receive! x)))
    (aset @websoc "onclose" #(.log js/console "Websocket closed"))
    (.log js/console "Websocket created")
    websoc))

(defonce websocket (init-websocket))
  ;; Launch the websocket


(def get-file
  ;; Transducer for channel uploaded-file that unpacks the useful values in
  ;; the event
  (map (fn [d] [(-> d (get 0) .-currentTarget .-files (aget 0))
                (get d 1)])))
(def uploaded-file (chan 1 get-file))

(def b64-conv
  ;; Transducer for channel b64-content that encodes the data as a b64 string
  (map (fn [d] [(as-> d x (get x 0) (.-target x) (.-result x)
                      (new js/Uint8Array x) (b64/encodeByteArray x))
                (get d 1)])))
(def b64-content (chan 1 b64-conv))

(defn greadloop! []
  "Read a file from an input channel (uploaded-file) and write its content to an output channel (b64-content)"
  (go-loop []
    (let [reader (js/FileReader.)
          [file id] (<! uploaded-file)]
      (.log js/console (str "Got file id " id " on the channel"))
      ;; (.log js/console file)
      (swap! upload assoc-in [id :file-name] (.-name file))
      (set! (.-onload reader) #(put! b64-content [% id]))
      (.readAsArrayBuffer reader file)
      (recur))))


(defn b64loop! []
  "Read data from an input channel(b64-content) and send its base64 conversion to the websocket"
  (go-loop []
    (let [[cont id] (<! b64-content)]
      (.log js/console (str "Got id " id " from the b64 channel"))
      (.log js/console cont)
      (.send @websocket (str "name:" id ":" (get-in @upload [id :file-name])))
      (.send @websocket (str "data:" id ":" cont))
      (swap! upload assoc-in [id :state] "done"))
      (recur)))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Initialize App

(defn dev-setup []
  (when ^boolean js/goog.DEBUG
    (enable-console-print!)
    (println "dev mode")))


(defn reload []
  (reagent/render [page upload queues logs errors result uploaded-file]
                  (.getElementById js/document "app")))

(defn ^:export main []
  "Start the loops and the web page"
  (dev-setup)
  (greadloop!)
  (b64loop!)
  (reload)
)
